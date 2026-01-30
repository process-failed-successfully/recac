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

	// Indices for deterministic lookup
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
			return nil
		}
		parsedFiles[path] = f

		pkgName := f.Name.Name
		dir := filepath.Dir(path)

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

		imports := make(map[string]string)
		for _, imp := range f.Imports {
			pathVal := strings.Trim(imp.Path.Value, "\"")
			var alias string
			if imp.Name != nil {
				alias = imp.Name.Name
			} else {
				parts := strings.Split(pathVal, "/")
				alias = parts[len(parts)-1]
			}
			imports[alias] = pathVal
		}
		fileImports[path] = imports

		for _, decl := range f.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok {
				node := &CallGraphNode{
					Package: fullPkg,
					Name:    fn.Name.Name,
				}

				if fn.Recv != nil {
					typeName := getReceiverTypeName(fn.Recv)
					node.Receiver = typeName
					node.ID = fmt.Sprintf("%s.(%s).%s", fullPkg, typeName, fn.Name.Name)
					methodIndex[fn.Name.Name] = append(methodIndex[fn.Name.Name], node)
				} else {
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

	// Prepare sorted package keys for deterministic iteration
	var sortedPkgKeys []string
	for pkg := range pkgIndex {
		sortedPkgKeys = append(sortedPkgKeys, pkg)
	}
	sort.Strings(sortedPkgKeys)

	// 2. Second Pass: Resolve Calls
	edgeMap := make(map[string]bool)

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

				ast.Inspect(fn.Body, func(n ast.Node) bool {
					call, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}

					var calleeID string

					switch fun := call.Fun.(type) {
					case *ast.Ident:
						candidateID := fmt.Sprintf("%s.%s", fullPkg, fun.Name)
						if _, exists := cg.Nodes[candidateID]; exists {
							calleeID = candidateID
						}
					case *ast.SelectorExpr:
						sel := fun.Sel.Name
						if xIdent, ok := fun.X.(*ast.Ident); ok {
							if importPath, isImport := imports[xIdent.Name]; isImport {
								calleeID = resolveExternalCall(pkgIndex, sortedPkgKeys, importPath, sel)
								if calleeID == "" {
									calleeID = fmt.Sprintf("%s.%s", importPath, sel)
								}
							} else {
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

func resolveExternalCall(pkgIndex map[string][]*CallGraphNode, sortedPkgKeys []string, importPath string, funcName string) string {
	var matches []string

	for _, pkg := range sortedPkgKeys {
		// Check if importPath ends with node.Package (which is pkg)
		// Ensure we match "pkg" or "/pkg", not just substring suffix
		suffix := "/" + pkg
		if importPath == pkg || strings.HasSuffix(importPath, suffix) {
			// Search nodes in this package
			nodes := pkgIndex[pkg]
			for _, node := range nodes {
				if node.Name == funcName {
					matches = append(matches, node.ID)
				}
			}
		}
	}

	if len(matches) == 0 {
		return ""
	}

	// Deterministic selection
	sort.Slice(matches, func(i, j int) bool {
		// Longest ID (Package) first
		if len(matches[i]) != len(matches[j]) {
			return len(matches[i]) > len(matches[j])
		}
		return matches[i] < matches[j]
	})

	return matches[0]
}

func findMethodsByName(methodIndex map[string][]*CallGraphNode, methodName string) []*CallGraphNode {
	return methodIndex[methodName]
}
