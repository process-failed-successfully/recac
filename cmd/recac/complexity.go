package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

type FunctionComplexity struct {
	File       string `json:"file"`
	Function   string `json:"function"`
	Complexity int    `json:"complexity"`
	Line       int    `json:"line"`
}

var (
	complexityThreshold int
	complexityJSON      bool
)

var complexityCmd = &cobra.Command{
	Use:   "complexity [path]",
	Short: "Calculate cyclomatic complexity of Go functions",
	Long:  `Calculates the cyclomatic complexity of Go functions in the specified path (defaulting to current directory). Displays functions exceeding the threshold.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		path := "."
		if len(args) > 0 {
			path = args[0]
		}

		results, err := runComplexityAnalysis(path)
		if err != nil {
			return err
		}

		// Filter by threshold
		var highComplexity []FunctionComplexity
		for _, res := range results {
			if res.Complexity >= complexityThreshold {
				highComplexity = append(highComplexity, res)
			}
		}

		// Sort by complexity (descending)
		sort.Slice(highComplexity, func(i, j int) bool {
			return highComplexity[i].Complexity > highComplexity[j].Complexity
		})

		if complexityJSON {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(highComplexity)
		}

		if len(highComplexity) == 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "No functions found with complexity >= %d. Good job!\n", complexityThreshold)
			return nil
		}

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "COMPLEXITY\tFUNCTION\tFILE:LINE")
		for _, res := range highComplexity {
			fmt.Fprintf(w, "%d\t%s\t%s:%d\n", res.Complexity, res.Function, res.File, res.Line)
		}
		w.Flush()

		return nil
	},
}

func init() {
	rootCmd.AddCommand(complexityCmd)
	complexityCmd.Flags().IntVar(&complexityThreshold, "threshold", 10, "Minimum complexity to report")
	complexityCmd.Flags().BoolVar(&complexityJSON, "json", false, "Output results as JSON")
}

func runComplexityAnalysis(root string) ([]FunctionComplexity, error) {
	var results []FunctionComplexity
	fset := token.NewFileSet()

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			// Don't skip the root directory itself if it's "."
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
			// Skip files that can't be parsed
			return nil
		}

		for _, decl := range node.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok {
				comp := calculateComplexity(fn)
				results = append(results, FunctionComplexity{
					File:       path,
					Function:   fn.Name.Name,
					Complexity: comp,
					Line:       fset.Position(fn.Pos()).Line,
				})
			}
		}

		return nil
	})

	return results, err
}

func calculateComplexity(fn *ast.FuncDecl) int {
	complexity := 1
	ast.Inspect(fn, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.CaseClause, *ast.CommClause:
			complexity++
		case *ast.BinaryExpr:
			if n.Op == token.LAND || n.Op == token.LOR {
				complexity++
			}
		}
		return true
	})
	return complexity
}
