package analysis

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

type ComplexityResult struct {
	File       string `json:"file"`
	Function   string `json:"function"`
	Complexity int    `json:"complexity"`
	Line       int    `json:"line"`
}

// CalculateComplexity computes the cyclomatic complexity of a function.
func CalculateComplexity(fn *ast.FuncDecl) int {
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

func RunComplexityAnalysis(root string) ([]ComplexityResult, error) {
	var results []ComplexityResult
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
				comp := CalculateComplexity(fn)
				results = append(results, ComplexityResult{
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
