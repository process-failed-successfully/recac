package breaking

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"path/filepath"
	"recac/internal/astutils"
	"strings"
)

// FileLoader is a function that retrieves file content.
type FileLoader func(path string) ([]byte, error)

// API represents the public interface of a codebase.
type API struct {
	// Identifiers maps exported names to their signatures/definitions.
	Identifiers map[string]string
}

// NewAPI creates an empty API.
func NewAPI() *API {
	return &API{
		Identifiers: make(map[string]string),
	}
}

// ExtractAPI analyzes Go files in the given paths using the loader.
func ExtractAPI(paths []string, loader FileLoader) (*API, error) {
	api := NewAPI()
	fset := token.NewFileSet()

	for _, path := range paths {
		// Basic filtering
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			continue
		}

		content, err := loader(path)
		if err != nil {
			return nil, fmt.Errorf("failed to load %s: %w", path, err)
		}

		f, err := parser.ParseFile(fset, path, content, parser.ParseComments)
		if err != nil {
			// Skip files that don't parse (might be templates or invalid)
			// Warning log would be nice but we don't have logger here
			continue
		}

		pkgName := f.Name.Name
		dir := filepath.Dir(path)

		for _, decl := range f.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Name.IsExported() {
					name := d.Name.Name
					if d.Recv != nil {
						typeName := astutils.GetReceiverTypeName(d.Recv)
						if typeName != "" {
							name = fmt.Sprintf("%s.%s", typeName, name)
						}
					}
					// Use printer to get signature
					key := fmt.Sprintf("%s/%s.%s", dir, pkgName, name)
					api.Identifiers[key] = nodeToString(fset, d.Type)
				}
			case *ast.GenDecl:
				if d.Tok == token.TYPE {
					for _, spec := range d.Specs {
						ts := spec.(*ast.TypeSpec)
						if ts.Name.IsExported() {
							// For structs/interfaces, we print the type spec to capture fields/methods
							key := fmt.Sprintf("%s/%s.%s", dir, pkgName, ts.Name.Name)
							api.Identifiers[key] = nodeToString(fset, ts.Type)
						}
					}
				} else if d.Tok == token.VAR || d.Tok == token.CONST {
					for _, spec := range d.Specs {
						vs := spec.(*ast.ValueSpec)
						for _, name := range vs.Names {
							if name.IsExported() {
								val := "var/const"
								if vs.Type != nil {
									val = nodeToString(fset, vs.Type)
								}
								key := fmt.Sprintf("%s/%s.%s", dir, pkgName, name.Name)
								api.Identifiers[key] = val
							}
						}
					}
				}
			}
		}
	}
	return api, nil
}

func nodeToString(fset *token.FileSet, node interface{}) string {
	var buf bytes.Buffer
	printer.Fprint(&buf, fset, node)
	// normalize spaces to avoid formatting noise
	s := strings.Join(strings.Fields(buf.String()), " ")
	return s
}
