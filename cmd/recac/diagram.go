package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var diagramCmd = &cobra.Command{
	Use:   "diagram [path]",
	Short: "Generate a class diagram from code",
	Long: `Generates a Mermaid class diagram by analyzing Go structs and their relationships.
This command parses the source code to identify structs, fields, and embeddings.`,
	RunE: runDiagram,
}

func init() {
	rootCmd.AddCommand(diagramCmd)
	diagramCmd.Flags().StringP("output", "o", "", "Output file path")
	diagramCmd.Flags().String("focus", "", "Regex to focus on specific structs by name")
	diagramCmd.Flags().Bool("fields", true, "Include fields in the diagram")
}

func runDiagram(cmd *cobra.Command, args []string) error {
	root := "."
	if len(args) > 0 {
		root = args[0]
	}

	focus, _ := cmd.Flags().GetString("focus")
	showFields, _ := cmd.Flags().GetBool("fields")
	outputFile, _ := cmd.Flags().GetString("output")

	// Compile focus regex if provided
	var focusRe *regexp.Regexp
	var err error
	if focus != "" {
		focusRe, err = regexp.Compile(focus)
		if err != nil {
			return fmt.Errorf("invalid focus regex: %w", err)
		}
	}

	// Analyze
	classes, relationships, err := analyzeStructs(root)
	if err != nil {
		return fmt.Errorf("analysis failed: %w", err)
	}

	// Generate Mermaid
	mermaid := generateMermaidClassDiagram(classes, relationships, focusRe, showFields)

	// Output
	if outputFile != "" {
		if err := os.WriteFile(outputFile, []byte(mermaid), 0644); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Diagram saved to %s\n", outputFile)
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), mermaid)
	}

	return nil
}

type ClassDef struct {
	Name    string
	Package string
	Fields  []string
}

type Relationship struct {
	From string
	To   string
	Type string // "embed", "has"
}

func analyzeStructs(root string) (map[string]*ClassDef, []Relationship, error) {
	classes := make(map[string]*ClassDef)
	var relationships []Relationship

	fset := token.NewFileSet()

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			name := info.Name()
			if name == ".git" || name == "vendor" || name == "node_modules" || name == ".recac" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			// Skip unparseable files gracefully
			return nil
		}

		pkgName := f.Name.Name

		ast.Inspect(f, func(n ast.Node) bool {
			ts, ok := n.(*ast.TypeSpec)
			if !ok {
				return true
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				return true
			}

			className := ts.Name.Name
			fullClassName := pkgName + "." + className
			// For simplicity, we might just use className if packages are unique, but let's stick to full for correctness internally, but maybe display short if unambigous?
			// Let's use simple names for display unless conflict, but store full.
			// Actually, Mermaid handles dots in class names with quotes, or we can sanitize.

			def := &ClassDef{
				Name:    className,
				Package: pkgName,
			}

			if st.Fields != nil {
				for _, field := range st.Fields.List {
					typeName := getTypeName(field.Type)

					// If no name, it's embedded
					if len(field.Names) == 0 {
						// Embed relationship
						// typeName might contain package
						target := typeName
						// Heuristic: if typeName doesn't have dot, assume same package
						if !strings.Contains(target, ".") {
							target = pkgName + "." + target
						}

						relationships = append(relationships, Relationship{
							From: fullClassName,
							To:   target, // This might be "pkg.Type" or just "Type" if local
							Type: "embed",
						})
						def.Fields = append(def.Fields, "<<"+typeName+">>")
					} else {
						for _, name := range field.Names {
							def.Fields = append(def.Fields, fmt.Sprintf("%s %s", typeName, name.Name))

							// Check if type is likely another struct we care about
							// Simple heuristic: starts with uppercase
							baseType := strings.TrimLeft(typeName, "[]*")
							if len(baseType) > 0 && (baseType[0] >= 'A' && baseType[0] <= 'Z') {
								target := baseType
								if !strings.Contains(target, ".") {
									target = pkgName + "." + target
								}
								relationships = append(relationships, Relationship{
									From: fullClassName,
									To:   target,
									Type: "has",
								})
							}
						}
					}
				}
			}

			classes[fullClassName] = def
			return false
		})

		return nil
	})

	return classes, relationships, err
}

func getTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + getTypeName(t.X)
	case *ast.SelectorExpr:
		return getTypeName(t.X) + "." + t.Sel.Name
	case *ast.ArrayType:
		return "[]" + getTypeName(t.Elt)
	case *ast.MapType:
		return "map[" + getTypeName(t.Key) + "]" + getTypeName(t.Value)
	default:
		return "unknown"
	}
}

func generateMermaidClassDiagram(classes map[string]*ClassDef, relationships []Relationship, focus *regexp.Regexp, showFields bool) string {
	var sb strings.Builder
	sb.WriteString("classDiagram\n")

	// Set of included classes (to filter relationships)
	included := make(map[string]bool)

	// Sort classes
	var keys []string
	for k := range classes {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, id := range keys {
		def := classes[id]
		if focus != nil && !focus.MatchString(def.Name) {
			continue
		}
		included[id] = true

		// Format: class Name {
		//    Type field
		// }

		// Sanitize ID for Mermaid (remove dots for ID, use label for display)
		safeID := sanitizeDiagramID(id)

		sb.WriteString(fmt.Sprintf("    class %s {\n", safeID))
		if showFields {
			for _, f := range def.Fields {
				// Escape quotes?
				safeF := strings.ReplaceAll(f, "\"", "'")
				sb.WriteString(fmt.Sprintf("        %s\n", safeF))
			}
		}
		sb.WriteString("    }\n")
	}

	// Relationships
	for _, rel := range relationships {
		// Resolve targets
		// Target might be "pkg.Type". We need to match it against our known classes keys.
		// Since we stored keys as "pkg.Type", exact match should work if analyzed correctly.
		// However, "has" relationship heuristics might produce "Type" (local) or "pkg.Type".
		// We normalized to "pkg.Type" in analysis if no dot.

		if !included[rel.From] {
			continue
		}

		// If target is not in our parsed classes, optionally skip or show as external?
		// Let's only show if target is in our list of classes to keep diagram clean,
		// unless we want to show external boundaries.
		// For now, strict check to avoid noise.
		if classes[rel.To] == nil && !included[rel.To] {
			// Check if we can find it (maybe we filtered it out via focus?)
			// If focus is active, we might want to show relationships to outside?
			// Let's only show if both sides are known struct definitions we found.
			if classes[rel.To] == nil {
				continue
			}
		}

		safeFrom := sanitizeDiagramID(rel.From)
		safeTo := sanitizeDiagramID(rel.To)

		// Avoid self-loops for "has" if it's just recursive type (e.g. Node *Node)
		if safeFrom == safeTo && rel.Type == "has" {
			// Maybe show it?
		}

		arrow := "-->"
		if rel.Type == "embed" {
			arrow = "*--"
		}

		sb.WriteString(fmt.Sprintf("    %s %s %s\n", safeFrom, arrow, safeTo))
	}

	return sb.String()
}

func sanitizeDiagramID(id string) string {
	return strings.ReplaceAll(id, ".", "_")
}
