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

// SequenceNode represents a function in the sequence analysis.
type SequenceNode struct {
	ID       string // "pkg.Func" or "pkg.(Type).Method"
	Package  string
	Name     string
	Receiver string
	Decl     *ast.FuncDecl
	Imports  map[string]string // Alias -> ImportPath
}

// GenerateSequence generates a Mermaid sequence diagram starting from the entry point.
func GenerateSequence(root string, entryPoint string, maxDepth int) (string, error) {
	fset := token.NewFileSet()
	nodes := make(map[string]*SequenceNode)

	// 1. Index all functions
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

		pkgName := f.Name.Name
		dir := filepath.Dir(path)
		relDir, _ := filepath.Rel(root, dir)
		fullPkg := relDir
		if relDir == "." {
			fullPkg = pkgName
		} else if filepath.Base(relDir) != pkgName {
			fullPkg = filepath.Join(relDir, pkgName)
		}
		fullPkg = strings.TrimPrefix(fullPkg, "./")
		fullPkg = strings.ReplaceAll(fullPkg, "\\", "/") // Normalize for Windows

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

		// Index Functions
		for _, decl := range f.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok {
				node := &SequenceNode{
					Package: fullPkg,
					Name:    fn.Name.Name,
					Decl:    fn,
					Imports: imports,
				}
				if fn.Recv != nil {
					typeName := getReceiverTypeName(fn.Recv)
					node.Receiver = typeName
					node.ID = fmt.Sprintf("%s.(%s).%s", fullPkg, typeName, fn.Name.Name)
				} else {
					node.ID = fmt.Sprintf("%s.%s", fullPkg, fn.Name.Name)
				}
				nodes[node.ID] = node
			}
		}
		return nil
	})

	if err != nil {
		return "", err
	}

	// 2. Find Entry Point
	var startNode *SequenceNode
	// Try exact match first
	if node, ok := nodes[entryPoint]; ok {
		startNode = node
	} else {
		// Try partial match (ignore package)
		var candidates []*SequenceNode
		uniqueCandidates := make(map[string]bool)

		for _, node := range nodes {
			matched := false
			if node.Name == entryPoint {
				matched = true
			}
			if strings.HasSuffix(node.ID, entryPoint) {
				matched = true
			}

			if matched && !uniqueCandidates[node.ID] {
				candidates = append(candidates, node)
				uniqueCandidates[node.ID] = true
			}
		}
		if len(candidates) == 1 {
			startNode = candidates[0]
		} else if len(candidates) > 1 {
			return "", fmt.Errorf("ambiguous entry point '%s', found %d matches (try using full ID like 'pkg.Func')", entryPoint, len(candidates))
		}
	}

	if startNode == nil {
		return "", fmt.Errorf("entry point '%s' not found", entryPoint)
	}

	// 3. Generate Sequence
	var sb strings.Builder
	sb.WriteString("sequenceDiagram\n")
	sb.WriteString("    autonumber\n")

	// Collect participants
	participants := make(map[string]bool)
	addParticipant := func(p string) {
		if !participants[p] {
			participants[p] = true
			safeP := sanitizeID(p)
			sb.WriteString(fmt.Sprintf("    participant %s as %s\n", safeP, p))
		}
	}

	// Recursive traversal
	var visit func(node *SequenceNode, depth int)
	visit = func(node *SequenceNode, depth int) {
		if depth > maxDepth {
			return
		}

		participant := node.Package
		if node.Receiver != "" {
			participant = fmt.Sprintf("%s.%s", node.Package, node.Receiver)
		}
		addParticipant(participant)

		if node.Decl.Body == nil {
			return
		}

		ast.Inspect(node.Decl.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			var calleeName string
			var calleeNode *SequenceNode

			switch fun := call.Fun.(type) {
			case *ast.Ident:
				// Local call
				candidateID := fmt.Sprintf("%s.%s", node.Package, fun.Name)
				if target, ok := nodes[candidateID]; ok {
					calleeNode = target
					calleeName = fun.Name
				} else {
					calleeName = fun.Name
				}

			case *ast.SelectorExpr:
				// X.Sel
				sel := fun.Sel.Name
				calleeName = sel

				if xIdent, ok := fun.X.(*ast.Ident); ok {
					// Check import alias
					if importPath, ok := node.Imports[xIdent.Name]; ok {
						// Match importPath against node packages
						// Heuristic: Check if any node package suffix matches import path
						for _, n := range nodes {
							if n.Name == sel && strings.HasSuffix(importPath, n.Package) {
								calleeNode = n
								break
							}
						}
						// If not found, it's external call to importPath
						if calleeNode == nil {
							// For external calls, we can't deep link, but we can show participant
							calleeName = fmt.Sprintf("%s.%s", xIdent.Name, sel)
						}
					}
				}
			}

			if calleeNode != nil {
				targetParticipant := calleeNode.Package
				if calleeNode.Receiver != "" {
					targetParticipant = fmt.Sprintf("%s.%s", calleeNode.Package, calleeNode.Receiver)
				}
				addParticipant(targetParticipant)

				safeCaller := sanitizeID(participant)
				safeTarget := sanitizeID(targetParticipant)

				sb.WriteString(fmt.Sprintf("    %s->>%s: %s()\n", safeCaller, safeTarget, calleeNode.Name))
				visit(calleeNode, depth+1)

			} else if calleeName != "" {
				// External or unknown call
			}

			return true
		})
	}

	visit(startNode, 1)

	return sb.String(), nil
}

func sanitizeID(id string) string {
	// Fast path: if no characters to replace, just return the original string
	if strings.IndexByte(id, '/') == -1 && strings.IndexByte(id, '.') == -1 && strings.IndexByte(id, '-') == -1 {
		return id
	}

	var sb strings.Builder
	sb.Grow(len(id))
	for i := 0; i < len(id); i++ {
		c := id[i]
		if c == '/' || c == '.' || c == '-' {
			sb.WriteByte('_')
		} else {
			sb.WriteByte(c)
		}
	}
	return sb.String()
}

// Ensure getReceiverTypeName is available.
// If callgraph.go is missing or in another package, un-commenting this would fix it.
// But we verified it exists in the same package.
/*
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
*/
