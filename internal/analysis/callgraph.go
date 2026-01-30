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

// CodebaseIndex holds indexed information about the codebase.
type CodebaseIndex struct {
	ParsedFiles map[string]*ast.File
	FileImports map[string]map[string]string // FilePath -> Alias -> ImportPath
	PkgIndex    map[string][]*CallGraphNode  // PackagePath -> Nodes (for faster lookup)
	MethodIndex map[string][]*CallGraphNode  // MethodName -> Nodes (for heuristic lookup)
}

// GenerateCallGraph analyzes the Go code in root and returns a call graph.
func GenerateCallGraph(root string) (*CallGraph, error) {
	fset := token.NewFileSet()
	cg := &CallGraph{
		Nodes: make(map[string]*CallGraphNode),
	}

	idx, err := indexCodebase(root, fset, cg)
	if err != nil {
		return nil, err
	}

	resolveCalls(cg, idx, root)

	return cg, nil
}

func indexCodebase(root string, fset *token.FileSet, cg *CallGraph) (*CodebaseIndex, error) {
	idx := &CodebaseIndex{
		ParsedFiles: make(map[string]*ast.File),
		FileImports: make(map[string]map[string]string),
		PkgIndex:    make(map[string][]*CallGraphNode),
		MethodIndex: make(map[string][]*CallGraphNode),
	}

	// Use WalkDir with sorted processing to ensure deterministic order if needed,
	// but WalkDir is lexical.
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
			return nil // Skip malformed
		}
		idx.ParsedFiles[path] = f

		pkgName := f.Name.Name
		fullPkg := determinePackagePath(root, path, pkgName)

		// Index Imports
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
		idx.FileImports[path] = imports

		// Index Functions
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
					idx.MethodIndex[fn.Name.Name] = append(idx.MethodIndex[fn.Name.Name], node)
				} else {
					node.ID = fmt.Sprintf("%s.%s", fullPkg, fn.Name.Name)
					idx.PkgIndex[fullPkg] = append(idx.PkgIndex[fullPkg], node)
				}

				cg.Nodes[node.ID] = node
			}
		}
		return nil
	})

	// Sort indices for determinism
	for _, nodes := range idx.PkgIndex {
		sortNodes(nodes)
	}
	for _, nodes := range idx.MethodIndex {
		sortNodes(nodes)
	}

	return idx, err
}

func sortNodes(nodes []*CallGraphNode) {
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].ID < nodes[j].ID
	})
}

func determinePackagePath(root, path, pkgName string) string {
	dir := filepath.Dir(path)
	relDir, err := filepath.Rel(root, dir)
	if err != nil || strings.HasPrefix(relDir, "..") {
		return pkgName
	}

	fullPkg := relDir
	if relDir == "." {
		fullPkg = pkgName
	} else if filepath.Base(relDir) != pkgName {
		fullPkg = filepath.Join(relDir, pkgName)
	}

	fullPkg = filepath.ToSlash(fullPkg)
	return strings.TrimPrefix(fullPkg, "./")
}

func resolveCalls(cg *CallGraph, idx *CodebaseIndex, root string) {
	edgeMap := make(map[string]bool)

	// Sort file paths for deterministic iteration order
	paths := make([]string, 0, len(idx.ParsedFiles))
	for p := range idx.ParsedFiles {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for _, path := range paths {
		f := idx.ParsedFiles[path]
		pkgName := f.Name.Name
		fullPkg := determinePackagePath(root, path, pkgName)
		imports := idx.FileImports[path]

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
						// Local call
						candidateID := fmt.Sprintf("%s.%s", fullPkg, fun.Name)
						if _, exists := cg.Nodes[candidateID]; exists {
							calleeID = candidateID
						}
					case *ast.SelectorExpr:
						// X.Sel()
						sel := fun.Sel.Name
						if xIdent, ok := fun.X.(*ast.Ident); ok {
							if importPath, isImport := imports[xIdent.Name]; isImport {
								calleeID = resolveExternalCall(idx, importPath, sel)
								if calleeID == "" {
									calleeID = fmt.Sprintf("%s.%s", importPath, sel)
								}
							} else {
								// Method on variable
								candidates := idx.MethodIndex[sel]
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

func resolveExternalCall(idx *CodebaseIndex, importPath string, funcName string) string {
	// Optimized lookup using PkgIndex

	// Direct match attempt
	if nodes, ok := idx.PkgIndex[importPath]; ok {
		for _, node := range nodes {
			if node.Name == funcName {
				return node.ID
			}
		}
	}

	// Suffix match
	var candidates []string
	for pkg := range idx.PkgIndex {
		// Strict suffix check: importPath must end with pkg
		// AND ensure boundary (slash or exact match)
		if importPath == pkg || strings.HasSuffix(importPath, "/"+pkg) {
			candidates = append(candidates, pkg)
		}
	}

	// If multiple candidates, pick the longest match (most specific)
	sort.Slice(candidates, func(i, j int) bool {
		return len(candidates[i]) < len(candidates[j])
	})

	for i := len(candidates) - 1; i >= 0; i-- {
		pkg := candidates[i]
		for _, node := range idx.PkgIndex[pkg] {
			if node.Name == funcName {
				return node.ID
			}
		}
	}

	return ""
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
