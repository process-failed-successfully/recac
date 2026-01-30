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

	parsedFiles, fileImports, err := indexDeclarations(root, fset, cg)
	if err != nil {
		return nil, err
	}

	resolveCalls(root, parsedFiles, fileImports, cg)

	// Sort edges for deterministic output
	sort.Slice(cg.Edges, func(i, j int) bool {
		if cg.Edges[i].From == cg.Edges[j].From {
			return cg.Edges[i].To < cg.Edges[j].To
		}
		return cg.Edges[i].From < cg.Edges[j].From
	})

	return cg, nil
}

func indexDeclarations(root string, fset *token.FileSet, cg *CallGraph) (map[string]*ast.File, map[string]map[string]string, error) {
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
		relDir, err := filepath.Rel(root, dir)
		if err != nil {
			relDir = "."
		}
		// Fix for Rel returning error or .. if root is absolute and dir is not or vice versa
		// But here we walk root so it should be fine.
		// Handling "." case
		fullPkg := relDir
		if relDir == "." {
			fullPkg = pkgName
		} else if filepath.Base(relDir) != pkgName {
			fullPkg = filepath.Join(relDir, pkgName)
		}
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
				} else {
					// Function
					node.ID = fmt.Sprintf("%s.%s", fullPkg, fn.Name.Name)
				}

				cg.Nodes[node.ID] = node
			}
		}
		return nil
	})

	return parsedFiles, fileImports, err
}

func resolveCalls(root string, parsedFiles map[string]*ast.File, fileImports map[string]map[string]string, cg *CallGraph) {
	edgeMap := make(map[string]bool)

	// Sort files for deterministic iteration
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
			relDir = "."
		}
		fullPkg := relDir
		if relDir == "." {
			fullPkg = pkgName
		} else if filepath.Base(relDir) != pkgName {
			fullPkg = filepath.Join(relDir, pkgName)
		}
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
							if importPath, isImport := imports[xIdent.Name]; isImport {
								// It is Pkg.Func()
								calleeID = resolveExternalCall(cg, importPath, sel)
								if calleeID == "" {
									calleeID = fmt.Sprintf("%s.%s", importPath, sel)
								}
							} else {
								// Variable.Method()
								candidates := findMethodsByName(cg, sel)
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
	// Handle generic types with multiple parameters (Go 1.18+)
	if indexList, ok := expr.(*ast.IndexListExpr); ok {
		if ident, ok := indexList.X.(*ast.Ident); ok {
			return ident.Name
		}
	}
	return "Unknown"
}

func resolveExternalCall(cg *CallGraph, importPath string, funcName string) string {
	var candidates []*CallGraphNode
	for _, node := range cg.Nodes {
		if node.Name == funcName && node.Receiver == "" {
			// Strict matching: importPath must match node.Package or end with "/node.Package"
			match := importPath == node.Package || strings.HasSuffix(importPath, "/"+node.Package)
			if match {
				candidates = append(candidates, node)
			}
		}
	}
	if len(candidates) == 0 {
		return ""
	}

	// Sort candidates:
	// 1. Longest Package path (most specific match)
	// 2. ID (lexicographically, for determinism)
	sort.Slice(candidates, func(i, j int) bool {
		lenI := len(candidates[i].Package)
		lenJ := len(candidates[j].Package)
		if lenI != lenJ {
			return lenI > lenJ // Descending length
		}
		return candidates[i].ID < candidates[j].ID
	})

	return candidates[0].ID
}

func findMethodsByName(cg *CallGraph, methodName string) []*CallGraphNode {
	var results []*CallGraphNode
	for _, node := range cg.Nodes {
		if node.Name == methodName && node.Receiver != "" {
			results = append(results, node)
		}
	}
	// Sort results for determinism
	sort.Slice(results, func(i, j int) bool {
		return results[i].ID < results[j].ID
	})
	return results
}
