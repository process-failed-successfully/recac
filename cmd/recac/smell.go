package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"recac/internal/analysis"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var (
	smellJSON       bool
	smellFail       bool
	smellThresholds = map[string]int{
		"loc":        50, // Lines of Code
		"params":     5,  // Parameter Count
		"returns":    3,  // Return Count
		"complexity": 10, // Cyclomatic Complexity
		"nesting":    4,  // Nesting Depth
	}
)

var smellCmd = &cobra.Command{
	Use:   "smell [path]",
	Short: "Detect code smells in Go functions",
	Long: `Analyzes Go functions for code smells including:
- High Cyclomatic Complexity
- Long Functions (LOC)
- Many Parameters
- Many Returns
- Deep Nesting

You can adjust thresholds via flags (e.g., --loc 100).`,
	RunE: runSmell,
}

type SmellFinding struct {
	File        string `json:"file"`
	Function    string `json:"function"`
	Line        int    `json:"line"`
	Type        string `json:"type"`
	Value       int    `json:"value"`
	Threshold   int    `json:"threshold"`
	Description string `json:"description"`
}

func init() {
	rootCmd.AddCommand(smellCmd)
	smellCmd.Flags().BoolVar(&smellJSON, "json", false, "Output results as JSON")
	smellCmd.Flags().BoolVar(&smellFail, "fail", false, "Exit with error code if smells are found")

	// Dynamic flags for thresholds
	smellCmd.Flags().Int("loc", 50, "Max Lines of Code")
	smellCmd.Flags().Int("params", 5, "Max Parameters")
	smellCmd.Flags().Int("returns", 3, "Max Returns")
	smellCmd.Flags().Int("complexity", 10, "Max Complexity")
	smellCmd.Flags().Int("nesting", 4, "Max Nesting Depth")
}

func runSmell(cmd *cobra.Command, args []string) error {
	path := "."
	if len(args) > 0 {
		path = args[0]
	}

	// Update thresholds from flags
	updateThreshold(cmd, "loc")
	updateThreshold(cmd, "params")
	updateThreshold(cmd, "returns")
	updateThreshold(cmd, "complexity")
	updateThreshold(cmd, "nesting")

	findings, err := analyzeSmells(path)
	if err != nil {
		return err
	}

	if smellJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(findings)
	}

	if len(findings) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "✅ No code smells found! Clean code.")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "TYPE\tFUNCTION\tVALUE\tFILE:LINE")
	for _, f := range findings {
		fmt.Fprintf(w, "%s\t%s\t%d (max %d)\t%s:%d\n", f.Type, f.Function, f.Value, f.Threshold, f.File, f.Line)
	}
	w.Flush()

	if smellFail {
		return fmt.Errorf("found %d code smells", len(findings))
	}

	return nil
}

func updateThreshold(cmd *cobra.Command, name string) {
	val, _ := cmd.Flags().GetInt(name)
	if val > 0 {
		smellThresholds[name] = val
	}
}

func analyzeSmells(root string) ([]SmellFinding, error) {
	var findings []SmellFinding
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
			return nil
		}

		analyzeNodeSmells(node, fset, path, &findings)

		return nil
	})

	return findings, err
}

func analyzeFileSmells(filename string, content []byte) ([]SmellFinding, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, content, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var findings []SmellFinding
	analyzeNodeSmells(node, fset, filename, &findings)
	return findings, nil
}

func analyzeNodeSmells(node *ast.File, fset *token.FileSet, filename string, findings *[]SmellFinding) {
	for _, decl := range node.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			metrics := analysis.AnalyzeFunction(fn, fset)
			checkMetrics(metrics, filename, findings)
		}
	}
}

func checkMetrics(m analysis.FunctionMetrics, file string, findings *[]SmellFinding) {
	if m.LOC > smellThresholds["loc"] {
		*findings = append(*findings, SmellFinding{
			File:      file,
			Function:  m.Name,
			Line:      m.Line,
			Type:      "Long Function",
			Value:     m.LOC,
			Threshold: smellThresholds["loc"],
		})
	}
	if m.ParameterCount > smellThresholds["params"] {
		*findings = append(*findings, SmellFinding{
			File:      file,
			Function:  m.Name,
			Line:      m.Line,
			Type:      "Many Parameters",
			Value:     m.ParameterCount,
			Threshold: smellThresholds["params"],
		})
	}
	if m.ReturnCount > smellThresholds["returns"] {
		*findings = append(*findings, SmellFinding{
			File:      file,
			Function:  m.Name,
			Line:      m.Line,
			Type:      "Many Returns",
			Value:     m.ReturnCount,
			Threshold: smellThresholds["returns"],
		})
	}
	if m.Complexity > smellThresholds["complexity"] {
		*findings = append(*findings, SmellFinding{
			File:      file,
			Function:  m.Name,
			Line:      m.Line,
			Type:      "Complex Function",
			Value:     m.Complexity,
			Threshold: smellThresholds["complexity"],
		})
	}
	if m.NestingDepth > smellThresholds["nesting"] {
		*findings = append(*findings, SmellFinding{
			File:      file,
			Function:  m.Name,
			Line:      m.Line,
			Type:      "Deep Nesting",
			Value:     m.NestingDepth,
			Threshold: smellThresholds["nesting"],
		})
	}
}
