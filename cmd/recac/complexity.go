package main

import (
	"encoding/json"
	"fmt"
	"recac/internal/analysis"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

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

		results, err := analysis.RunComplexityAnalysis(path)
		if err != nil {
			return err
		}

		// Filter by threshold
		var highComplexity []analysis.ComplexityResult
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
