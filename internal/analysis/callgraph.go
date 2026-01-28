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

	// Store parsed files to avoid re-parsing
	parsedFiles := make(map[string]*ast.File)

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") && name != "." {
				return filepath.SkipDir
			}
			if name == "vendor" || name == "node_modules" || name == "testdata" {
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
				func() {
					defer func() {
						recover() // Ignore panic during indexing to continue
					}()

					node := &CallGraphNode{
						Package: fullPkg,
						Name:    fn.Name.Name,
					}

					if fn.Recv != nil {
						// Method
						typeName := getReceiverTypeName(fn.Recv)
						node.Receiver = typeName
						node.ID = fmt.Sprintf("%s.(%s).%s", fullPkg, typeName, fn.Name.Name)
					} else {
						// Function
						node.ID = fmt.Sprintf("%s.%s", fullPkg, fn.Name.Name)
					}

					cg.Nodes[node.ID] = node
				}()
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Prepare sorted lists for deterministic second pass
	var sortedFiles []string
	for path := range parsedFiles {
		sortedFiles = append(sortedFiles, path)
	}
	sort.Strings(sortedFiles)

	var sortedNodes []*CallGraphNode
	for _, node := range cg.Nodes {
		sortedNodes = append(sortedNodes, node)
	}
	sort.Slice(sortedNodes, func(i, j int) bool {
		return sortedNodes[i].ID < sortedNodes[j].ID
	})

	// 2. Second Pass: Resolve Calls
	// Use map to prevent duplicates
	edgeMap := make(map[string]bool)

	for _, path := range sortedFiles {
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
				func() {
					defer func() {
						recover() // Ignore panic during analysis to continue
					}()

					var callerID string
					if fn.Recv != nil {
						callerID = fmt.Sprintf("%s.(%s).%s", fullPkg, getReceiverTypeName(fn.Recv), fn.Name.Name)
					} else {
						callerID = fmt.Sprintf("%s.%s", fullPkg, fn.Name.Name)
					}

					// Inspect body
					if fn.Body == nil {
						return // Skip forward declarations or assembly
					}
					ast.Inspect(fn.Body, func(n ast.Node) bool {
						call, ok := n.(*ast.CallExpr)
						if !ok {
							return true
						}

						var calleeID string

						switch fun := call.Fun.(type) {
						case *ast.Ident:
							// Local call: DoSomething()
							// Likely same package, simple function
							candidateID := fmt.Sprintf("%s.%s", fullPkg, fun.Name)
							if _, exists := cg.Nodes[candidateID]; exists {
								calleeID = candidateID
							} else {
								// Could be a method on 'this' implicitly? No, Go doesn't allow implicit 'this'.
								// Must be a builtin or definition missing.
							}

						case *ast.SelectorExpr:
							// X.Sel()
							sel := fun.Sel.Name

							if xIdent, ok := fun.X.(*ast.Ident); ok {
								// Ident.Sel()
								// Check if Ident is a package import
								if importPath, isImport := imports[xIdent.Name]; isImport {
									// It is Pkg.Func()
									// We need to match the package path structure we used for keys.
									// We used "dir/pkgName". External imports won't match our local keys unless we handle external packages.
									// For now, let's assume we only graph INTERNAL calls or we use a fallback ID.

									// Try to find if we have nodes with this Package
									// This is tricky because "importPath" is like "github.com/foo/bar"
									// But our keys are "internal/bar.Func".
									// We will try to match suffix.
									calleeID = resolveExternalCall(sortedNodes, importPath, sel)
									if calleeID == "" {
										// Treat as external node
										calleeID = fmt.Sprintf("%s.%s", importPath, sel)
									}
								} else {
									// Variable.Method()
									// We don't know the type of Variable.
									// Heuristic: Find ANY method named 'Sel' in our codebase.
									candidates := findMethodsByName(sortedNodes, sel)
									if len(candidates) == 1 {
										calleeID = candidates[0].ID
									} else if len(candidates) > 1 {
										// Ambiguous. We can leave empty or point to a special "ambiguous" node.
										// For now, let's skip or mark as ambiguous?
										// Let's create an edge to the method name generic node?
										// Or just pick one?
										// Better: Create edges to ALL candidates but mark them as "heuristic" (dashed)?
										// For simplicity in this v1:
										// Create a "virtual" node for the method if we can't resolve.
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
				}()
			}
		}
	}

	// Sort edges for determinism
	sort.Slice(cg.Edges, func(i, j int) bool {
		if cg.Edges[i].From != cg.Edges[j].From {
			return cg.Edges[i].From < cg.Edges[j].From
		}
		return cg.Edges[i].To < cg.Edges[j].To
	})

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

func resolveExternalCall(nodes []*CallGraphNode, importPath string, funcName string) string {
	// Our nodes are keyed by "relDir/pkg.Func".
	// Import path is "recac/internal/foo".
	// If we are running on "recac" repo, "internal/foo" matches.

	// Normalize import path
	// Remove module prefix if possible?
	// This is hard without knowing module name.
	// But we can scan all nodes and check if Node.Package matches the end of ImportPath?

	for _, node := range nodes {
		if node.Name == funcName && node.Receiver == "" {
			// Check if importPath ends with node.Package
			// node.Package might be "internal/utils"
			// importPath might be "recac/internal/utils"
			if strings.HasSuffix(importPath, node.Package) {
				return node.ID
			}
		}
	}
	return ""
}

func findMethodsByName(nodes []*CallGraphNode, methodName string) []*CallGraphNode {
	var results []*CallGraphNode
	for _, node := range nodes {
		if node.Name == methodName && node.Receiver != "" {
			results = append(results, node)
		}
	}
	return results
}
