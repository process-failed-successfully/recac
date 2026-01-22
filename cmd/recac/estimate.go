package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	estimateFocus string
	estimateJson  bool
)

// EstimateResult represents the structured output from the AI
type EstimateResult struct {
	Summary             string   `json:"summary"`
	Complexity          string   `json:"complexity"`      // Low, Medium, High
	StoryPoints         int      `json:"story_points"`    // Fibonacci: 1, 2, 3, 5, 8, 13, 21
	EstimatedHours      string   `json:"estimated_hours"` // e.g. "2-4h"
	Risks               []string `json:"risks"`
	ImplementationSteps []string `json:"implementation_steps"`
}

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

	// 3. Construct Prompt
	prompt := fmt.Sprintf(`You are a pragmatic Senior Software Engineer.
Your goal is to ESTIMATE the effort required for the following task.

Task: "%s"

%s

Provide a realistic estimation. Be conservative. Consider testing, documentation, and potential side effects.

Return the result as a raw JSON object with the following structure:
{
  "summary": "Brief summary of the approach",
  "complexity": "Low|Medium|High",
  "story_points": <integer_fibonacci_sequence>,
  "estimated_hours": "range (e.g. 4-6h)",
  "risks": ["risk 1", "risk 2"],
  "implementation_steps": ["step 1", "step 2"]
}

Do not wrap the JSON in markdown code blocks. Just return the raw JSON string.`,
		taskDescription,
		func() string {
			if codebaseContext != "" {
				return "Context Codebase:\n" + codebaseContext
			}
			return "No specific code context provided. Base estimate on general best practices."
		}())

	fmt.Fprintln(cmd.OutOrStdout(), "🤖 Crunching numbers (this may take a moment)...")

	// 4. Send to Agent
	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed to generate estimate: %w", err)
	}

	// 5. Parse Response
	jsonStr := utils.CleanJSONBlock(resp)
	var result EstimateResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		// Fallback: Just print raw output if parsing fails
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Failed to parse JSON response: %v\n", err)
		fmt.Fprintln(cmd.OutOrStdout(), "\nRaw Response:")
		fmt.Fprintln(cmd.OutOrStdout(), resp)
		return nil
	}

	// 6. Output
	if estimateJson {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	printEstimateReport(cmd, result)
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
