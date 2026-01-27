package analysis

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
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

	parsedFiles, fileImports, err := parseAndIndexFiles(root, fset, cg)
	if err != nil {
		return nil, err
	}

	resolveCalls(root, parsedFiles, fileImports, cg)

	return cg, nil
}

func parseAndIndexFiles(root string, fset *token.FileSet, cg *CallGraph) (map[string]*ast.File, map[string]map[string]string, error) {
	parsedFiles := make(map[string]*ast.File)
	fileImports := make(map[string]map[string]string)

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

		fullPkg := getApproximatePackagePath(root, path, f.Name.Name)

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

	for path, f := range parsedFiles {
		fullPkg := getApproximatePackagePath(root, path, f.Name.Name)
		imports := fileImports[path]

		for _, decl := range f.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok {
				processFunctionDecl(fn, fullPkg, imports, cg, edgeMap)
			}
		}
	}
}

func processFunctionDecl(fn *ast.FuncDecl, fullPkg string, imports map[string]string, cg *CallGraph, edgeMap map[string]bool) {
	var callerID string
	if fn.Recv != nil {
		callerID = fmt.Sprintf("%s.(%s).%s", fullPkg, getReceiverTypeName(fn.Recv), fn.Name.Name)
	} else {
		callerID = fmt.Sprintf("%s.%s", fullPkg, fn.Name.Name)
	}

	if fn.Body == nil {
		return
	}

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		calleeID := resolveCallee(call, fullPkg, imports, cg)

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

func resolveCallee(call *ast.CallExpr, fullPkg string, imports map[string]string, cg *CallGraph) string {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		// Local call: DoSomething()
		candidateID := fmt.Sprintf("%s.%s", fullPkg, fun.Name)
		if _, exists := cg.Nodes[candidateID]; exists {
			return candidateID
		}

	case *ast.SelectorExpr:
		// X.Sel()
		sel := fun.Sel.Name

		if xIdent, ok := fun.X.(*ast.Ident); ok {
			// Ident.Sel()
			if importPath, isImport := imports[xIdent.Name]; isImport {
				calleeID := resolveExternalCall(cg, importPath, sel)
				if calleeID == "" {
					// Treat as external node
					return fmt.Sprintf("%s.%s", importPath, sel)
				}
				return calleeID
			} else {
				// Variable.Method()
				// Heuristic: Find ANY method named 'Sel' in our codebase.
				candidates := findMethodsByName(cg, sel)
				if len(candidates) == 1 {
					return candidates[0].ID
				} else if len(candidates) > 1 {
					return fmt.Sprintf("(Ambiguous).%s", sel)
				}
			}
		}
	}
	return ""
}

func getApproximatePackagePath(root, path, pkgName string) string {
	dir := filepath.Dir(path)
	relDir, _ := filepath.Rel(root, dir)
	fullPkg := relDir
	if relDir == "." {
		fullPkg = pkgName
	} else if filepath.Base(relDir) != pkgName {
		fullPkg = filepath.Join(relDir, pkgName)
	}
	return strings.TrimPrefix(fullPkg, "./")
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

func resolveExternalCall(cg *CallGraph, importPath string, funcName string) string {
	for id, node := range cg.Nodes {
		if node.Name == funcName && node.Receiver == "" {
			if importPath == node.Package || strings.HasSuffix(importPath, "/"+node.Package) {
				return id
			}
		}
	}
	return ""
}

func findMethodsByName(cg *CallGraph, methodName string) []*CallGraphNode {
	var results []*CallGraphNode
	for _, node := range cg.Nodes {
		if node.Name == methodName && node.Receiver != "" {
			results = append(results, node)
		}
	}
	return results
}
