package analysis

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
)

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

func AnalyzeStructs(root string) (map[string]*ClassDef, []Relationship, error) {
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

func GenerateMermaidClassDiagram(classes map[string]*ClassDef, relationships []Relationship, focus *regexp.Regexp, showFields bool) string {
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
		if !included[rel.From] {
			continue
		}

		if classes[rel.To] == nil && !included[rel.To] {
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
