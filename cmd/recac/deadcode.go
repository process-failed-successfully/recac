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
	fset := token.NewFileSet()
	var files []*ast.File
	var filePaths []string
	ignoredDirs := DefaultIgnoreMap()

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if ignoredDirs[info.Name()] {
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
		files = append(files, f)
		filePaths = append(filePaths, path)
		return nil
	})
	if err != nil {
		return nil, err
	}

	declarations := collectDeclarations(files, filePaths, fset)
	usagesCount := collectUsages(files)

	var results []DeadcodeFinding
	for name, list := range declarations {
		if name == "main" || name == "init" {
			continue
		}

		checkName := name
		if strings.Contains(name, ".") {
			parts := strings.Split(name, ".")
			checkName = parts[1]
		}

		if usagesCount[checkName] == 0 {
			if checkName == "String" || checkName == "Error" {
				continue
			}

			if !deadcodeStrict && len(list) > 0 {
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

func collectDeclarations(files []*ast.File, filePaths []string, fset *token.FileSet) map[string][]DeadcodeFinding {
	declarations := make(map[string][]DeadcodeFinding)
	for i, f := range files {
		path := filePaths[i]
		packageName := f.Name.Name

		for _, decl := range f.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Name.IsExported() {
					name := d.Name.Name
					key := name
					if d.Recv != nil {
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
				}
			}
		}
	}
	return declarations
}

func collectUsages(files []*ast.File) map[string]int {
	usagesCount := make(map[string]int)
	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			if n == nil {
				return true
			}
			switch x := n.(type) {
			case *ast.FuncDecl:
				if x.Type != nil {
					walk(x.Type, usagesCount)
				}
				if x.Body != nil {
					walk(x.Body, usagesCount)
				}
				return false
			case *ast.TypeSpec:
				walk(x.Type, usagesCount)
				return false
			case *ast.ValueSpec:
				if x.Type != nil {
					walk(x.Type, usagesCount)
				}
				for _, v := range x.Values {
					walk(v, usagesCount)
				}
				return false
			case *ast.Field:
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
	return usagesCount
}

func walk(node ast.Node, usages map[string]int) {
	ast.Inspect(node, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok {
			usages[ident.Name]++
		}
		return true
	})
}
