package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"recac/internal/analysis"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var (
	optCiJSON   bool
	optCiIgnore string
)

var optimizeCiCmd = &cobra.Command{
	Use:   "optimize-ci [path]",
	Short: "Analyze CI/CD configurations for security and performance",
	Long: `Scans GitHub Actions workflows (YAML files) for common best practices,
security risks, and optimization opportunities.

Checks:
- Immutable action references (SHAs)
- Job timeouts
- Least privilege permissions
- Caching for setup actions`,
	RunE: runOptimizeCi,
}

func init() {
	rootCmd.AddCommand(optimizeCiCmd)
	optimizeCiCmd.Flags().BoolVar(&optCiJSON, "json", false, "Output results as JSON")
	optimizeCiCmd.Flags().StringVar(&optCiIgnore, "ignore", "", "Comma-separated list of rule IDs to ignore")
}

func runOptimizeCi(cmd *cobra.Command, args []string) error {
	target := ".github/workflows"
	if len(args) > 0 {
		target = args[0]
	}

	info, err := os.Stat(target)
	if err != nil {
		if os.IsNotExist(err) && len(args) == 0 {
			// If default target doesn't exist, just warn and exit
			fmt.Fprintln(cmd.OutOrStdout(), "Default directory .github/workflows not found. Please specify a file or directory.")
			return nil
		}
		return fmt.Errorf("failed to access %s: %w", target, err)
	}

	var allFindings []struct {
		File string `json:"file"`
		analysis.CIFinding
	}

	ignores := make(map[string]bool)
	if optCiIgnore != "" {
		for _, rule := range strings.Split(optCiIgnore, ",") {
			ignores[strings.TrimSpace(rule)] = true
		}
	}

	// Walk function
	walkFn := func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".yml") && !strings.HasSuffix(path, ".yaml") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		findings, err := analysis.AnalyzeGitHubWorkflow(string(content))
		if err != nil {
			// Warn but continue
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to parse %s: %v\n", path, err)
			return nil
		}

		for _, f := range findings {
			if !ignores[f.Rule] {
				allFindings = append(allFindings, struct {
					File string `json:"file"`
					analysis.CIFinding
				}{File: path, CIFinding: f})
			}
		}
		return nil
	}

	if info.IsDir() {
		err = filepath.WalkDir(target, walkFn)
	} else {
		// Single file
		err = filepath.WalkDir(target, walkFn)
	}

	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	if optCiJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(allFindings)
	}

	if len(allFindings) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "✅ No issues found! CI configurations are optimized.")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "SEVERITY\tFILE:LINE\tRULE\tMESSAGE")
	for _, f := range allFindings {
		icon := "ℹ️"
		if f.Severity == "warning" {
			icon = "⚠️"
		} else if f.Severity == "error" {
			icon = "❌"
		}
		fmt.Fprintf(w, "%s %s\t%s:%d\t%s\t%s\n", icon, strings.ToUpper(f.Severity), f.File, f.Line, f.Rule, f.Message)
	}
	w.Flush()

	// Print unique advice
	fmt.Fprintln(cmd.OutOrStdout(), "\n💡 Advice:")
	printedAdvice := make(map[string]bool)
	for _, f := range allFindings {
		key := f.Rule + f.Advice
		if !printedAdvice[key] {
			fmt.Fprintf(cmd.OutOrStdout(), "- %s: %s\n", f.Rule, f.Advice)
			printedAdvice[key] = true
		}
	}

	return nil
}
