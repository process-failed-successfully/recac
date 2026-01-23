package main

import (
	"encoding/json"
	"fmt"
	"os"
	"recac/internal/analysis"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var (
	optDockerFile   string
	optDockerJSON   bool
	optDockerIgnore string
)

var optimizeDockerCmd = &cobra.Command{
	Use:   "optimize-docker",
	Short: "Analyze Dockerfile for optimization and security issues",
	Long: `Parses a Dockerfile and checks for common best practices,
security risks, and optimization opportunities (e.g., layer caching, image size).`,
	RunE: runOptimizeDocker,
}

func init() {
	optimizeDockerCmd.Flags().StringVarP(&optDockerFile, "file", "f", "Dockerfile", "Path to Dockerfile")
	optimizeDockerCmd.Flags().BoolVar(&optDockerJSON, "json", false, "Output results as JSON")
	optimizeDockerCmd.Flags().StringVar(&optDockerIgnore, "ignore", "", "Comma-separated list of rule IDs to ignore")
	rootCmd.AddCommand(optimizeDockerCmd)
}

func runOptimizeDocker(cmd *cobra.Command, args []string) error {
	content, err := os.ReadFile(optDockerFile)
	if err != nil {
		return fmt.Errorf("failed to read Dockerfile: %w", err)
	}

	findings, err := analysis.AnalyzeDockerfile(string(content))
	if err != nil {
		return err
	}

	// Filter ignores
	var filtered []analysis.DockerFinding
	ignores := make(map[string]bool)
	if optDockerIgnore != "" {
		for _, rule := range strings.Split(optDockerIgnore, ",") {
			ignores[strings.TrimSpace(rule)] = true
		}
	}

	for _, f := range findings {
		if !ignores[f.Rule] {
			filtered = append(filtered, f)
		}
	}

	if optDockerJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(filtered)
	}

	if len(filtered) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "✅ No issues found! Dockerfile is optimized.")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "SEVERITY\tLINE\tRULE\tMESSAGE")
	for _, f := range filtered {
		icon := "ℹ️"
		if f.Severity == "warning" {
			icon = "⚠️"
		} else if f.Severity == "error" {
			icon = "❌"
		}
		fmt.Fprintf(w, "%s %s\t%d\t%s\t%s\n", icon, strings.ToUpper(f.Severity), f.Line, f.Rule, f.Message)
	}
	w.Flush()

	// Print advice
	fmt.Fprintln(cmd.OutOrStdout(), "\n💡 Advice:")
	for _, f := range filtered {
		fmt.Fprintf(cmd.OutOrStdout(), "- %s (Line %d): %s\n", f.Rule, f.Line, f.Advice)
	}

	return nil
}
