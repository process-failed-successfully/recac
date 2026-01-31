package analysis

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

// CallGraphNode represents a function or method in the graph.
type CallGraphNode struct {
	ID       string // Unique ID: "pkg.Func" or "pkg.(Type).Method"
	Package  string
	Name     string
	Receiver string // Empty if function, TypeName if method
}

// CallGraphEdge represents a call from one node to another.
type CallGraphEdge struct {
	From string
	To   string
}

// CallGraph contains the nodes and edges of the analysis.
type CallGraph struct {
	Nodes map[string]*CallGraphNode
	Edges []CallGraphEdge
}

// GenerateCallGraph analyzes the Go code in root and returns a call graph.
func GenerateCallGraph(root string) (*CallGraph, error) {
	fset := token.NewFileSet()
	cg := &CallGraph{
		Nodes: make(map[string]*CallGraphNode),
	}

	// Indices for lookup
	pkgIndex := make(map[string][]*CallGraphNode)    // PackagePath -> Nodes
	methodIndex := make(map[string][]*CallGraphNode) // MethodName -> Nodes

	// 1. First Pass: Index all functions and methods
	fileImports := make(map[string]map[string]string)
	parsedFiles := make(map[string]*ast.File)

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") && d.Name() != "." {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			// Skip malformed files
			return nil
		}
		parsedFiles[path] = f

		pkgName := f.Name.Name
		dir := filepath.Dir(path)

		// Approximate full package path
		relDir, _ := filepath.Rel(root, dir)
		fullPkg := relDir
		if relDir == "." {
			fullPkg = pkgName
		} else if filepath.Base(relDir) != pkgName {
			fullPkg = filepath.Join(relDir, pkgName)
		}
		// Ensure slash separator for consistent IDs
		fullPkg = filepath.ToSlash(fullPkg)
		fullPkg = strings.TrimPrefix(fullPkg, "./")

		// Index Imports
		imports := make(map[string]string)
		for _, imp := range f.Imports {
			pathVal := strings.Trim(imp.Path.Value, "\"")
			var alias string
			if imp.Name != nil {
				alias = imp.Name.Name
			} else {
				// Default alias is last part of path
				parts := strings.Split(pathVal, "/")
				alias = parts[len(parts)-1]
			}
			imports[alias] = pathVal
		}
		fileImports[path] = imports

		// Index Functions
		for _, decl := range f.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok {
				node := &CallGraphNode{
					Package: fullPkg,
					Name:    fn.Name.Name,
				}

				if fn.Recv != nil {
					// Method
					typeName := getReceiverTypeName(fn.Recv)
					node.Receiver = typeName
					node.ID = fmt.Sprintf("%s.(%s).%s", fullPkg, typeName, fn.Name.Name)
					methodIndex[fn.Name.Name] = append(methodIndex[fn.Name.Name], node)
				} else {
					// Function
					node.ID = fmt.Sprintf("%s.%s", fullPkg, fn.Name.Name)
					pkgIndex[fullPkg] = append(pkgIndex[fullPkg], node)
				}

				cg.Nodes[node.ID] = node
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	// 2. Second Pass: Resolve Calls
	// Use map to prevent duplicates
	edgeMap := make(map[string]bool)

	// Deterministic iteration over files
	var sortedPaths []string
	for p := range parsedFiles {
		sortedPaths = append(sortedPaths, p)
	}
	sort.Strings(sortedPaths)

	for _, path := range sortedPaths {
		f := parsedFiles[path]
		pkgName := f.Name.Name
		dir := filepath.Dir(path)
		relDir, _ := filepath.Rel(root, dir)
		fullPkg := relDir
		if relDir == "." {
			fullPkg = pkgName
		} else if filepath.Base(relDir) != pkgName {
			fullPkg = filepath.Join(relDir, pkgName)
		}
		fullPkg = filepath.ToSlash(fullPkg)
		fullPkg = strings.TrimPrefix(fullPkg, "./")

		imports := fileImports[path]

		for _, decl := range f.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok {
				var callerID string
				if fn.Recv != nil {
					callerID = fmt.Sprintf("%s.(%s).%s", fullPkg, getReceiverTypeName(fn.Recv), fn.Name.Name)
				} else {
					callerID = fmt.Sprintf("%s.%s", fullPkg, fn.Name.Name)
				}

				// Inspect body
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					call, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}

					var calleeID string

					switch fun := call.Fun.(type) {
					case *ast.Ident:
						// Local call: DoSomething()
						candidateID := fmt.Sprintf("%s.%s", fullPkg, fun.Name)
						if _, exists := cg.Nodes[candidateID]; exists {
							calleeID = candidateID
						} else {
							// Could be a method on 'this' implicitly? No, Go doesn't allow implicit 'this'.
						}

					case *ast.SelectorExpr:
						// X.Sel()
						sel := fun.Sel.Name

						if xIdent, ok := fun.X.(*ast.Ident); ok {
							// Ident.Sel()
							// Check if Ident is a package import
							if importPath, isImport := imports[xIdent.Name]; isImport {
								// It is Pkg.Func()
								// Use improved resolveExternalCall
								calleeID = resolveExternalCall(pkgIndex, importPath, sel)
								if calleeID == "" {
									// Treat as external node
									calleeID = fmt.Sprintf("%s.%s", importPath, sel)
								}
							} else {
								// Variable.Method()
								candidates := findMethodsByName(methodIndex, sel)
								if len(candidates) == 1 {
									calleeID = candidates[0].ID
								} else if len(candidates) > 1 {
									calleeID = fmt.Sprintf("(Ambiguous).%s", sel)
								}
							}
						}
					}

					if calleeID != "" {
						edgeKey := callerID + "->" + calleeID
						if !edgeMap[edgeKey] {
							cg.Edges = append(cg.Edges, CallGraphEdge{
								From: callerID,
								To:   calleeID,
							})
							edgeMap[edgeKey] = true
						}
					}

					return true
				})
			}
		}
	}

	return cg, nil
}

func getReceiverTypeName(recv *ast.FieldList) string {
	if len(recv.List) == 0 {
		return ""
	}
	expr := recv.List[0].Type
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name
	}
	if index, ok := expr.(*ast.IndexExpr); ok {
		if ident, ok := index.X.(*ast.Ident); ok {
			return ident.Name
		}
	}
	return "Unknown"
}

func resolveExternalCall(pkgIndex map[string][]*CallGraphNode, importPath string, funcName string) string {
	// Find all matching packages
	var candidates []string

	for pkg, nodes := range pkgIndex {
		// pkg is "internal/foo"
		// importPath is "github.com/recac/internal/foo"
		// Match suffix
		// IMPORTANT: Match strict suffix (starts with / or is full match)
		// e.g. "pkg/a" matches "example.com/pkg/a"
		// "pkg/a" should NOT match "example.com/sub/pkg/a" if "sub/pkg/a" exists?
		// But here we just find candidates.

		if strings.HasSuffix(importPath, pkg) {
			// Check if boundary is correct
			// importPath: .../pkg
			// suffix: pkg
			// diff: .../
			// It must be empty or end in /
			prefixLen := len(importPath) - len(pkg)
			if prefixLen == 0 || importPath[prefixLen-1] == '/' {
				for _, node := range nodes {
					if node.Name == funcName {
						candidates = append(candidates, node.ID)
					}
				}
			}
		}
	}

	if len(candidates) == 0 {
		return ""
	}

	// Deterministic selection:
	// 1. Longest ID length (proxy for longest package match)
	// 2. Lexicographically first
	sort.Slice(candidates, func(i, j int) bool {
		if len(candidates[i]) != len(candidates[j]) {
			return len(candidates[i]) > len(candidates[j])
		}
		return candidates[i] < candidates[j]
	})

	return candidates[0]
}

func findMethodsByName(methodIndex map[string][]*CallGraphNode, methodName string) []*CallGraphNode {
	return methodIndex[methodName]
}
