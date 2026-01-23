package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var (
	deadcodeJSON   bool
	deadcodeFail   bool
	deadcodeStrict bool
)

var deadcodeCmd = &cobra.Command{
	Use:   "deadcode [path]",
	Short: "Detect unused code in Go packages",
	Long: `Analyzes Go packages to find unused exported functions and types.
By default, it checks for exported identifiers in a main package that are not used.
With --strict, it reports all exported identifiers that seem unused in the current scope.
Note: This is a static analysis heuristic and may have false positives for libraries.`,
	RunE: runDeadcode,
}

type DeadcodeFinding struct {
	File        string `json:"file"`
	Line        int    `json:"line"`
	Identifier  string `json:"identifier"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

func init() {
	rootCmd.AddCommand(deadcodeCmd)
	deadcodeCmd.Flags().BoolVar(&deadcodeJSON, "json", false, "Output results as JSON")
	deadcodeCmd.Flags().BoolVar(&deadcodeFail, "fail", false, "Exit with error code if findings are detected")
	deadcodeCmd.Flags().BoolVar(&deadcodeStrict, "strict", false, "Enable strict mode (report more potential unused exports)")
}

func runDeadcode(cmd *cobra.Command, args []string) error {
	path := "."
	if len(args) > 0 {
		path = args[0]
	}

	findings, err := analyzeDeadcode(path)
	if err != nil {
		return err
	}

	if deadcodeJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(findings)
	}

	if len(findings) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "✅ No dead code found!")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "TYPE\tIDENTIFIER\tFILE:LINE\tDESCRIPTION")
	for _, f := range findings {
		fmt.Fprintf(w, "%s\t%s\t%s:%d\t%s\n", f.Type, f.Identifier, f.File, f.Line, f.Description)
	}
	w.Flush()

	if deadcodeFail {
		return fmt.Errorf("found %d unused identifiers", len(findings))
	}

	return nil
}

func analyzeDeadcode(root string) ([]DeadcodeFinding, error) {
	// 1. Collect all exported declarations
	// 2. Collect all usages
	// 3. Diff

	fset := token.NewFileSet()
	declarations := make(map[string][]DeadcodeFinding) // name -> []locations

	// We need to handle package scopes. For simplicity in this heuristic version:
	// We assume unique names or just check globally.
	// Better: keyed by Package.Name
	// But simple heuristic: "If ExportedFunc is never called in any other file, it might be dead"

	// First pass: parse all files
	var files []*ast.File
	var filePaths []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") || info.Name() == "vendor" || info.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil // ignore parse errors
		}
		files = append(files, f)
		filePaths = append(filePaths, path)
		return nil
	})
	if err != nil {
		return nil, err
	}

	for i, f := range files {
		path := filePaths[i]
		packageName := f.Name.Name

		// Collect declarations
		for _, decl := range f.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Name.IsExported() {
					name := d.Name.Name
					// If it's a method, we might want to qualify it? Receiver type?
					// For now, simple func names.
					// Method: Type.Func
					key := name
					if d.Recv != nil {
						// It's a method
						// Try to get receiver type name
						for _, field := range d.Recv.List {
							typeExpr := field.Type
							if star, ok := typeExpr.(*ast.StarExpr); ok {
								typeExpr = star.X
							}
							if ident, ok := typeExpr.(*ast.Ident); ok {
								key = ident.Name + "." + name
							}
						}
					}

					declarations[key] = append(declarations[key], DeadcodeFinding{
						File:        path,
						Line:        fset.Position(d.Pos()).Line,
						Identifier:  key,
						Type:        "Function",
						Description: fmt.Sprintf("Exported function %s in package %s is never used", key, packageName),
					})
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					if typeSpec, ok := spec.(*ast.TypeSpec); ok {
						if typeSpec.Name.IsExported() {
							key := typeSpec.Name.Name
							declarations[key] = append(declarations[key], DeadcodeFinding{
								File:        path,
								Line:        fset.Position(typeSpec.Pos()).Line,
								Identifier:  key,
								Type:        "Type",
								Description: fmt.Sprintf("Exported type %s in package %s is never used", key, packageName),
							})
						}
					}
					// Handle variables/constants? Maybe later.
				}
			}
		}
	}

	var results []DeadcodeFinding

	// Usage scan
	usagesCount := make(map[string]int)

	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			if n == nil {
				return true
			}
			switch x := n.(type) {
			case *ast.FuncDecl:
				// Skip the function name definition
				// But traverse the body/params
				// SKIP Receiver type: defining a method on a type doesn't mean the type is "used" in the application logic sense.
				// It just means the type has methods.
				if x.Type != nil {
					walk(x.Type, usagesCount)
				}
				if x.Body != nil {
					walk(x.Body, usagesCount)
				}
				return false
			case *ast.TypeSpec:
				// Skip name
				walk(x.Type, usagesCount)
				return false
			case *ast.ValueSpec:
				// Skip Names
				if x.Type != nil {
					walk(x.Type, usagesCount)
				}
				for _, v := range x.Values {
					walk(v, usagesCount)
				}
				return false
			case *ast.Field:
				// Skip Names (parameter names, struct field names)
				walk(x.Type, usagesCount)
				if x.Tag != nil {
					walk(x.Tag, usagesCount)
				}
				return false
			case *ast.Ident:
				usagesCount[x.Name]++
			}
			return true
		})
	}

	for name, list := range declarations {
		if name == "main" || name == "init" {
			continue
		}

		// Split Method
		checkName := name
		if strings.Contains(name, ".") {
			parts := strings.Split(name, ".")
			checkName = parts[1] // Method name
		}

		if usagesCount[checkName] == 0 {
			// Special cases
			if checkName == "String" || checkName == "Error" {
				continue
			}

			// If not strict, assume all exported symbols in non-main packages might be used by external consumers
			// But wait, we don't know if a package is "main" or "lib" easily per declaration,
			// except checking the file's package declaration.
			// Let's check the package name of the first finding (all findings for this key should be in same package if unique)
			if !deadcodeStrict && len(list) > 0 {
				// We can check if the file belongs to "package main"
				// But we lost that context in the map, except it is in `Description` or we can re-parse.
				// Actually, `Description` says "... in package %s".
				// Or we can just check if package name is "main".
				// Let's parse it from description or just trust the user uses --strict for libs.
				// Simpler: if any finding for this symbol is in a "package main", then strictness applies by default (it MUST be used).
				// If it is in a library package, we only report if --strict is on.

				isMain := strings.Contains(list[0].Description, "package main")
				if !isMain {
					continue
				}
			}

			results = append(results, list...)
		}
	}

	return results, nil
}

func walk(node ast.Node, usages map[string]int) {
	ast.Inspect(node, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok {
			usages[ident.Name]++
		}
		return true
	})
}
