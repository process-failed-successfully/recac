package main

import (
	"encoding/json"
	"fmt"
	"recac/internal/analysis"
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

	findings, err := analysis.AnalyzeDeadcode(path, deadcodeStrict)
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
