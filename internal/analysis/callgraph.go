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

	// 1. First Pass: Index all functions and methods
	// We also need to track imports per file to resolve calls later.
	// Map: FilePath -> ImportMap (Alias -> PkgPath)
	fileImports := make(map[string]map[string]string)

	// Indices for faster and deterministic resolution
	pkgIndex := make(map[string][]*CallGraphNode)     // PackagePath -> Nodes
	methodIndex := make(map[string][]*CallGraphNode) // MethodName -> Nodes

	// Store parsed files to avoid re-parsing
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
		relDir, err := filepath.Rel(root, dir)
		if err != nil {
			return nil
		}
		fullPkg := relDir
		if relDir == "." {
			fullPkg = pkgName
		} else if filepath.Base(relDir) != pkgName {
			fullPkg = filepath.Join(relDir, pkgName)
		}
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
				}

				cg.Nodes[node.ID] = node
				pkgIndex[fullPkg] = append(pkgIndex[fullPkg], node)
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Sort indices for determinism
	for _, nodes := range pkgIndex {
		sort.Slice(nodes, func(i, j int) bool {
			return nodes[i].ID < nodes[j].ID
		})
	}
	for _, nodes := range methodIndex {
		sort.Slice(nodes, func(i, j int) bool {
			return nodes[i].ID < nodes[j].ID
		})
	}

	// 2. Second Pass: Resolve Calls
	// Use map to prevent duplicates
	edgeMap := make(map[string]bool)

	// Sort files for determinism
	var paths []string
	for p := range parsedFiles {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for _, path := range paths {
		f := parsedFiles[path]
		pkgName := f.Name.Name
		dir := filepath.Dir(path)
		relDir, err := filepath.Rel(root, dir)
		if err != nil {
			continue
		}
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
						}

					case *ast.SelectorExpr:
						// X.Sel()
						sel := fun.Sel.Name

						if xIdent, ok := fun.X.(*ast.Ident); ok {
							// Ident.Sel()
							// Check if Ident is a package import
							if importPath, isImport := imports[xIdent.Name]; isImport {
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
	var matches []*CallGraphNode

	for pkg, nodes := range pkgIndex {
		// Check if importPath ends with pkg
		// Ensure we match "pkg" or "/pkg", not just substring suffix
		suffix := "/" + pkg
		if importPath == pkg || strings.HasSuffix(importPath, suffix) {
			for _, node := range nodes {
				if node.Name == funcName && node.Receiver == "" {
					matches = append(matches, node)
				}
			}
		}
	}

	if len(matches) == 0 {
		return ""
	}

	// Deterministic selection: prefer longest match (most specific), then alphabetical
	sort.Slice(matches, func(i, j int) bool {
		pkgI := matches[i].Package
		pkgJ := matches[j].Package
		if len(pkgI) != len(pkgJ) {
			return len(pkgI) > len(pkgJ) // Longest package path first
		}
		return matches[i].ID < matches[j].ID
	})

	return matches[0].ID
}

func findMethodsByName(methodIndex map[string][]*CallGraphNode, methodName string) []*CallGraphNode {
	// methodIndex is already sorted by ID
	return methodIndex[methodName]
}
