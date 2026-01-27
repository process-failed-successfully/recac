package analysis

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// MagicFinding represents a found magic literal
type MagicFinding struct {
	Value       string   `json:"value"`
	Type        string   `json:"type"` // INT, FLOAT, STRING
	Occurrences int      `json:"occurrences"`
	Locations   []string `json:"locations"` // "file.go:12"
}

// ExtractMagicLiterals scans the root directory for magic literals
func ExtractMagicLiterals(root string, ignoreList []string) ([]MagicFinding, error) {
	// Map to store occurrences: Value -> []Location
	occurrences := make(map[string][]string)
	// Map to store type: Value -> Type
	types := make(map[string]string)

	ignores := make(map[string]bool)
	for _, i := range ignoreList {
		ignores[i] = true
	}
	// Default ignores
	defaultIgnores := []string{"0", "1", "-1", "\"\"", "true", "false", "nil"}
	for _, i := range defaultIgnores {
		ignores[i] = true
	}

	fset := token.NewFileSet()

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			if (strings.HasPrefix(info.Name(), ".") && info.Name() != ".") || info.Name() == "vendor" || info.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}

		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			// Just skip files that don't parse
			return nil
		}

		// Use a custom walker to handle context (like inside const or struct tag)
		v := &magicVisitor{
			fset:        fset,
			path:        path,
			occurrences: occurrences,
			types:       types,
			ignores:     ignores,
			inConst:     false,
		}
		ast.Walk(v, node)

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Convert map to slice
	var findings []MagicFinding
	for val, locs := range occurrences {
		findings = append(findings, MagicFinding{
			Value:       val,
			Type:        types[val],
			Occurrences: len(locs),
			Locations:   locs,
		})
	}

	return findings, nil
}

type magicVisitor struct {
	fset        *token.FileSet
	path        string
	occurrences map[string][]string
	types       map[string]string
	ignores     map[string]bool
	inConst     bool
}

func (v *magicVisitor) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		return nil
	}

	switch n := node.(type) {
	case *ast.GenDecl:
		if n.Tok == token.CONST {
			// Return a NEW visitor for the children of this node, with inConst=true.
			// The parent's visitor 'v' remains with inConst=false.
			return &magicVisitor{
				fset:        v.fset,
				path:        v.path,
				occurrences: v.occurrences,
				types:       v.types,
				ignores:     v.ignores,
				inConst:     true,
			}
		}
	case *ast.Field:
		// Handle Field traversal manually to skip Tag
		if n.Doc != nil {
			ast.Walk(v, n.Doc)
		}
		for _, name := range n.Names {
			ast.Walk(v, name)
		}
		ast.Walk(v, n.Type)
		// Skip Tag
		if n.Comment != nil {
			ast.Walk(v, n.Comment)
		}
		return nil // We handled children manually
	case *ast.BasicLit:
		if v.inConst {
			return nil
		}

		val := n.Value
		// Normalize string literals: remove quotes? No, keep them to distinguish "1" from 1
		// But maybe consistent quoting?

		if v.ignores[val] {
			return nil
		}

		// Check valid types
		if n.Kind != token.INT && n.Kind != token.FLOAT && n.Kind != token.STRING {
			return nil
		}

		loc := fmt.Sprintf("%s:%d", v.path, v.fset.Position(n.Pos()).Line)
		v.occurrences[val] = append(v.occurrences[val], loc)
		v.types[val] = n.Kind.String()
		return nil
	case *ast.ImportSpec:
		return nil // Don't visit imports
	}

	return v
}
