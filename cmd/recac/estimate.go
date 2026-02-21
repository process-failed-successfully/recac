package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	estimateFocus string
	estimateJson  bool
)

var estimateCmd = &cobra.Command{
	Use:   "estimate [task description]",
	Short: "Estimate complexity and effort for a task using AI",
	Long: `Uses AI to analyze a proposed task and provide an estimation of complexity, time, and risks.
You can focus the analysis on specific parts of the codebase to get a more grounded estimate.

Example:
  recac estimate "Refactor the login logic to use JWT" --focus internal/auth
  recac estimate "Add a new endpoint for user profile"`,
	Args: cobra.MinimumNArgs(1),
	RunE: runEstimate,
}

func init() {
	rootCmd.AddCommand(estimateCmd)
	estimateCmd.Flags().StringVarP(&estimateFocus, "focus", "f", "", "File or directory to provide as context")
	estimateCmd.Flags().BoolVar(&estimateJson, "json", false, "Output results as JSON")
}

func runEstimate(cmd *cobra.Command, args []string) error {
	taskDescription := strings.Join(args, " ")
	ctx := cmd.Context()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	// 1. Generate Context (if focus is provided)
	var codebaseContext string
	if estimateFocus != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "🔍 Analyzing context from %s...\n", estimateFocus)
		opts := ContextOptions{
			Roots:     []string{estimateFocus},
			MaxSize:   100 * 1024,
			Tree:      true,
			NoContent: false,
		}
		codebaseContext, err = GenerateCodebaseContext(opts)
		if err != nil {
			return fmt.Errorf("failed to generate codebase context: %w", err)
		}
	}

	// 2. Prepare Agent
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-estimate")
	if err != nil {
		return fmt.Errorf("failed to initialize agent: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "🤖 Crunching numbers (this may take a moment)...")

	// 3. Estimate
	rawResp, result, err := EstimateTaskWithAgent(ctx, ag, taskDescription, codebaseContext)
	if err != nil {
		// Fallback: Just print raw output if parsing fails (but we have a response)
		if rawResp != "" {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Failed to parse JSON response: %v\n", err)
			fmt.Fprintln(cmd.OutOrStdout(), "\nRaw Response:")
			fmt.Fprintln(cmd.OutOrStdout(), rawResp)
			return nil
		}
		return err
	}

	// 4. Output
	if estimateJson {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	printEstimateReport(cmd, *result)
	return nil
}

func printEstimateReport(cmd *cobra.Command, res EstimateResult) {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "\nESTIMATION REPORT")
	fmt.Fprintln(w, "-----------------")

	// Icons
	compIcon := "🟢"
	if res.Complexity == "Medium" {
		compIcon = "🟡"
	} else if res.Complexity == "High" {
		compIcon = "🔴"
	}

	fmt.Fprintf(w, "Complexity:\t%s %s\n", compIcon, res.Complexity)
	fmt.Fprintf(w, "Story Points:\t%d\n", res.StoryPoints)
	fmt.Fprintf(w, "Est. Hours:\t%s\n", res.EstimatedHours)
	fmt.Fprintln(w, "")
	w.Flush()

	fmt.Fprintln(cmd.OutOrStdout(), "Summary:")
	fmt.Fprintf(cmd.OutOrStdout(), "  %s\n\n", res.Summary)

	if len(res.Risks) > 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "⚠️  Risks:")
		for _, r := range res.Risks {
			fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", r)
		}
		fmt.Println("")
	}

	if len(res.ImplementationSteps) > 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "📋 Implementation Plan:")
		for i, step := range res.ImplementationSteps {
			fmt.Fprintf(cmd.OutOrStdout(), "  %d. %s\n", i+1, step)
		}
		fmt.Println("")
	}
}
