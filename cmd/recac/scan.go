package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"recac/internal/analysis/todo"
	"recac/internal/db"
	"recac/internal/utils"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// ScanResult is an alias for the shared todo.Item type.
type ScanResult = todo.Item

var (
	scanJSON       bool
	scanExportPlan string
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan codebase for technical debt markers",
	Long:  `Scans the current directory recursively for TODO, FIXME, BUG, HACK, and XXX markers.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		results, err := runScan(".")
		if err != nil {
			return err
		}

		if len(results) == 0 {
			if !scanJSON {
				fmt.Fprintln(cmd.OutOrStdout(), "No markers found. Great job!")
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "[]")
			}
			return nil
		}

		if scanExportPlan != "" {
			return exportPlan(results, scanExportPlan)
		}

		if scanJSON {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(results)
		}

		printTable(cmd, results)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(scanCmd)
	scanCmd.Flags().BoolVar(&scanJSON, "json", false, "Output results as JSON")
	scanCmd.Flags().StringVar(&scanExportPlan, "export-plan", "", "Export results as a feature list plan to the specified file (e.g., plan.json)")
}

func runScan(root string) ([]ScanResult, error) {
	return todo.Scan(root, utils.DefaultIgnoreMap())
}

func printTable(cmd *cobra.Command, results []ScanResult) {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "TYPE\tFILE\tLINE\tMESSAGE")
	for _, r := range results {
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\n", r.Keyword, r.File, r.Line, r.Content)
	}
	w.Flush()
}

func exportPlan(results []ScanResult, outputFile string) error {
	var features []db.Feature
	projectName := filepath.Base(outputFile)
	if projectName == "." || projectName == "/" {
		cwd, _ := os.Getwd()
		projectName = filepath.Base(cwd)
	}

	for i, r := range results {
		feature := db.Feature{
			ID:          fmt.Sprintf("tech-debt-%d", i+1),
			Category:    "Technical Debt",
			Priority:    "Low", // Default to low
			Status:      "pending",
			Description: fmt.Sprintf("%s in %s:%d: %s", r.Keyword, r.File, r.Line, r.Content),
			Steps: []string{
				fmt.Sprintf("Locate the %s marker in %s at line %d", r.Keyword, r.File, r.Line),
				"Analyze the context and determine the necessary changes",
				"Implement the fix or improvement",
				"Remove the marker",
			},
		}

		if r.Keyword == "BUG" || r.Keyword == "FIXME" {
			feature.Priority = "Medium"
		}

		features = append(features, feature)
	}

	plan := db.FeatureList{
		ProjectName: projectName,
		Features:    features,
	}

	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal plan: %w", err)
	}

	if err := os.WriteFile(outputFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write plan file: %w", err)
	}

	fmt.Printf("Successfully exported %d items to %s\n", len(features), outputFile)
	return nil
}
