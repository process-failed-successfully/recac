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

	// Map: PackageName -> PkgPath (approximate, relative to root)
	// We'll use "dir/pkg" as PkgPath for simplicity.

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
			relDir = "."
		}
		fullPkg := relDir
		if relDir == "." {
			fullPkg = pkgName
		} else if filepath.Base(relDir) != pkgName {
			fullPkg = filepath.Join(relDir, pkgName)
		}
		// Ensure forward slashes for package paths (fix for Windows)
		fullPkg = filepath.ToSlash(strings.TrimPrefix(fullPkg, "./"))

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

	if err != nil {
		return nil, err
	}

	// 2. Second Pass: Resolve Calls
	// Use map to prevent duplicates
	edgeMap := make(map[string]bool)

	// Sort file paths for deterministic iteration
	var sortedPaths []string
	for path := range parsedFiles {
		sortedPaths = append(sortedPaths, path)
	}
	sort.Strings(sortedPaths)

	for _, path := range sortedPaths {
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
		// Ensure forward slashes for package paths (fix for Windows)
		fullPkg = filepath.ToSlash(strings.TrimPrefix(fullPkg, "./"))

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
								calleeID = resolveExternalCall(cg, importPath, sel)
								if calleeID == "" {
									// Treat as external node
									calleeID = fmt.Sprintf("%s.%s", importPath, sel)
								}
							} else {
								// Variable.Method()
								// We don't know the type of Variable.
								// Heuristic: Find ANY method named 'Sel' in our codebase.
								candidates := findMethodsByName(cg, sel)
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
	if indexList, ok := expr.(*ast.IndexListExpr); ok {
		if ident, ok := indexList.X.(*ast.Ident); ok {
			return ident.Name
		}
	}
	return "Unknown"
}

func resolveExternalCall(cg *CallGraph, importPath string, funcName string) string {
	// Our nodes are keyed by "relDir/pkg.Func".
	// Import path is "recac/internal/foo".
	// If we are running on "recac" repo, "internal/foo" matches.

	// Normalize import path
	// Remove module prefix if possible?
	// This is hard without knowing module name.
	// But we can scan all nodes and check if Node.Package matches the end of ImportPath?

	// Sort nodes for determinism
	var candidateIDs []string
	for id, node := range cg.Nodes {
		if node.Name == funcName && node.Receiver == "" {
			// Check if importPath ends with node.Package
			// We check for Exact match OR Suffix match with separator to avoid false positives
			// e.g. "foo" should not match "internal/foo" unless importPath is ".../internal/foo"
			// But here we compare importPath (full) with node.Package (local relative).

			// Case 1: node.Package == "internal/foo", importPath == "recac/internal/foo"
			// Suffix match "internal/foo" is correct.

			// Case 2: node.Package == "foo", importPath == "recac/internal/foo"
			// Suffix match "foo" is correct IF foo is the package name.
			// But wait, if importPath is "recac/internal/foo", the package name is likely "foo".
			// But node.Package is the RELATIVE path from root.

			// If node.Package is "internal/foo", and importPath ends with "internal/foo", it's a match.
			// If node.Package is "foo" (in root), and importPath ends with "foo", it's a match.

			// We must ensure we match a full path component.
			if importPath == node.Package || strings.HasSuffix(importPath, "/"+node.Package) {
				candidateIDs = append(candidateIDs, id)
			}
		}
	}

	sort.Strings(candidateIDs)

	if len(candidateIDs) > 0 {
		// Pick the longest match? Or just the first one?
		// If we have "foo" and "internal/foo", and import is "recac/internal/foo".
		// "internal/foo" is a better match than "foo".
		// So we should pick the one with the longest Package string.

		bestID := candidateIDs[0]
		maxLen := len(cg.Nodes[bestID].Package)

		for _, id := range candidateIDs {
			if len(cg.Nodes[id].Package) > maxLen {
				bestID = id
				maxLen = len(cg.Nodes[id].Package)
			}
		}
		return bestID
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
	// Sort results by ID for determinism
	sort.Slice(results, func(i, j int) bool {
		return results[i].ID < results[j].ID
	})
	return results
}
