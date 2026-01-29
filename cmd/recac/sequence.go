package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var (
	sequenceDepth int
	sequenceDir   string
)

var sequenceCmd = &cobra.Command{
	Use:   "sequence [function_name]",
	Short: "Generate a Mermaid sequence diagram for a specific function",
	Long: `Generates a Mermaid sequence diagram by tracing function calls starting from a specific function.
This command performs a depth-first search of the call graph, preserving the order of calls within functions.

Example:
  recac sequence main.main
  recac sequence --depth 3 --dir ./internal/agent Agent.Run`,
	Args: cobra.ExactArgs(1),
	RunE: runSequence,
}

func init() {
	rootCmd.AddCommand(sequenceCmd)
	sequenceCmd.Flags().IntVarP(&sequenceDepth, "depth", "d", 5, "Maximum depth of the sequence diagram")
	sequenceCmd.Flags().StringVar(&sequenceDir, "dir", ".", "Directory to analyze")
}

func runSequence(cmd *cobra.Command, args []string) error {
	startFunc := args[0]

	// 1. Parse Codebase
	index, err := buildFunctionIndex(sequenceDir)
	if err != nil {
		return fmt.Errorf("failed to parse codebase: %w", err)
	}

	// 2. Find Start Node
	// We support fuzzy matching if exact match fails
	startNodeID := resolveFunctionID(index, startFunc)
	if startNodeID == "" {
		return fmt.Errorf("function '%s' not found", startFunc)
	}

	// 3. Generate Diagram
	diagram := generateMermaidSequence(index, startNodeID, sequenceDepth)

	fmt.Fprintln(cmd.OutOrStdout(), diagram)
	return nil
}

type FuncNode struct {
	ID       string // pkg.Func or pkg.(Type).Func
	Package  string
	Name     string
	Receiver string // Empty if function, TypeName if method
	Decl     *ast.FuncDecl
	Imports  map[string]string // Alias -> Path
}

func buildFunctionIndex(root string) (map[string]*FuncNode, error) {
	index := make(map[string]*FuncNode)
	fset := token.NewFileSet()

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

		pkgName := f.Name.Name
		dir := filepath.Dir(path)

		// Approximate package path relative to root
		relDir, _ := filepath.Rel(root, dir)
		fullPkg := relDir
		if relDir == "." {
			fullPkg = pkgName
		} else if filepath.Base(relDir) != pkgName {
			// e.g. internal/agent (package agent) -> internal/agent
			fullPkg = filepath.Join(relDir, pkgName)
		}
		fullPkg = strings.TrimPrefix(fullPkg, "./")
		fullPkg = strings.ReplaceAll(fullPkg, "\\", "/") // Normalize for Windows

		// Parse Imports
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

		for _, decl := range f.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok {
				var id string
				if fn.Recv != nil {
					typeName := getReceiverTypeNameSeq(fn.Recv)
					id = fmt.Sprintf("%s.(%s).%s", fullPkg, typeName, fn.Name.Name)
				} else {
					id = fmt.Sprintf("%s.%s", fullPkg, fn.Name.Name)
				}

				receiver := ""
				if fn.Recv != nil {
					receiver = getReceiverTypeNameSeq(fn.Recv)
				}
				index[id] = &FuncNode{
					ID:       id,
					Package:  fullPkg,
					Name:     fn.Name.Name,
					Receiver: receiver,
					Decl:     fn,
					Imports:  imports,
				}
			}
		}
		return nil
	})

	return index, err
}

// getReceiverTypeNameSeq extracts the receiver type name (copied to avoid internal dependency issues)
func getReceiverTypeNameSeq(recv *ast.FieldList) string {
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
	// Generic type instantiation?
	return "Unknown"
}

func resolveFunctionID(index map[string]*FuncNode, query string) string {
	if _, exists := index[query]; exists {
		return query
	}
	// Try fuzzy match
	// 1. Match suffix (e.g. "Run" -> "internal/agent.(Agent).Run")
	var matches []string
	for id := range index {
		if strings.HasSuffix(id, query) {
			matches = append(matches, id)
		}
	}
	if len(matches) == 1 {
		return matches[0]
	}
	// If multiple or none, return empty (ambiguous)
	// Could improve to pick shortest match or similar
	return ""
}

func generateMermaidSequence(index map[string]*FuncNode, startID string, maxDepth int) string {
	var sb strings.Builder
	sb.WriteString("sequenceDiagram\n")
	sb.WriteString("    autonumber\n")

	visited := make(map[string]bool) // Current stack for cycle detection

	// Start with an external actor calling the start function?
	// Or just show the start function doing things?
	// Let's assume an "User" or "Caller" calls the start function.
	safeStart := sanitizeSequenceID(startID)
	sb.WriteString(fmt.Sprintf("    User->>%s: %s()\n", safeStart, index[startID].Name))
	sb.WriteString(fmt.Sprintf("    activate %s\n", safeStart))

	generateSequenceRecursive(&sb, index, startID, maxDepth, 1, visited)

	sb.WriteString(fmt.Sprintf("    %s-->>User: return\n", safeStart))
	sb.WriteString(fmt.Sprintf("    deactivate %s\n", safeStart))

	return sb.String()
}

func generateSequenceRecursive(sb *strings.Builder, index map[string]*FuncNode, currentID string, maxDepth, currentDepth int, stack map[string]bool) {
	if currentDepth >= maxDepth {
		return
	}

	node, exists := index[currentID]
	if !exists {
		return
	}

	stack[currentID] = true
	defer func() { stack[currentID] = false }()

	ast.Inspect(node.Decl.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		calleeID := resolveCall(index, node, call)
		if calleeID == "" {
			return true
		}

		safeCurrent := sanitizeSequenceID(currentID)
		safeCallee := sanitizeSequenceID(calleeID)

		calleeName := calleeID
		if target, ok := index[calleeID]; ok {
			calleeName = target.Name
		} else {
			// Clean up ID for display
			parts := strings.Split(calleeID, ".")
			calleeName = parts[len(parts)-1]
		}

		// Check recursion
		if stack[calleeID] {
			sb.WriteString(fmt.Sprintf("    %s->>%s: %s() (Recursive)\n", safeCurrent, safeCallee, calleeName))
			// Don't recurse
		} else {
			sb.WriteString(fmt.Sprintf("    %s->>%s: %s()\n", safeCurrent, safeCallee, calleeName))
			sb.WriteString(fmt.Sprintf("    activate %s\n", safeCallee))

			generateSequenceRecursive(sb, index, calleeID, maxDepth, currentDepth+1, stack)

			sb.WriteString(fmt.Sprintf("    %s-->>%s: return\n", safeCallee, safeCurrent))
			sb.WriteString(fmt.Sprintf("    deactivate %s\n", safeCallee))
		}

		return true
	})
}

func resolveCall(index map[string]*FuncNode, caller *FuncNode, call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		// Local call: func()
		candidateID := fmt.Sprintf("%s.%s", caller.Package, fun.Name)
		if _, exists := index[candidateID]; exists {
			return candidateID
		}
		// Might be a builtin or not found

	case *ast.SelectorExpr:
		// X.Sel()
		sel := fun.Sel.Name
		if xIdent, ok := fun.X.(*ast.Ident); ok {
			// Check if X is an import
			if importPath, isImport := caller.Imports[xIdent.Name]; isImport {
				// External package call: importPath.Sel
				// We need to match how we indexed it.
				// Our index uses "relDir/pkg".
				// importPath might be "github.com/org/repo/internal/pkg"
				// We try suffix matching against all nodes? No, too slow.
				// Just try to construct a candidate ID if we assume it's internal.

				// Heuristic: If importPath ends with a package in our index, use it.
				for id, node := range index {
					if node.Name == sel && node.Receiver == "" {
						// Check if importPath ends with node.Package
						if strings.HasSuffix(importPath, node.Package) {
							return id
						}
					}
				}
				// If not found in index, return it as external node
				return fmt.Sprintf("%s.%s", importPath, sel)
			}

			// X is a variable/type. We don't know the type easily.
			// Try to find any method with name Sel?
			// This is ambiguous.
			// Let's try to find an exact match if only one exists in index?
			var candidates []string
			for id, node := range index {
				if node.Name == sel && node.Receiver != "" {
					candidates = append(candidates, id)
				}
			}
			if len(candidates) == 1 {
				return candidates[0]
			}
			// If ambiguous, return a special ID
			if len(candidates) > 0 {
				return fmt.Sprintf("(Ambiguous).%s", sel)
			}
		}
	}
	return ""
}

func sanitizeSequenceID(id string) string {
	id = strings.ReplaceAll(id, "(", "_")
	id = strings.ReplaceAll(id, ")", "_")
	id = strings.ReplaceAll(id, "*", "ptr_")
	id = strings.ReplaceAll(id, ".", "_")
	id = strings.ReplaceAll(id, "/", "_")
	id = strings.ReplaceAll(id, "-", "_")
	id = strings.ReplaceAll(id, " ", "_")
	return id
}
