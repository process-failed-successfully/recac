package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"recac/internal/runner"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	planCmd.Flags().String("path", ".", "Project path to provide context for planning")
	viper.BindPFlag("plan.path", planCmd.Flags().Lookup("path"))
	rootCmd.AddCommand(planCmd)
}

var planCmd = &cobra.Command{
	Use:   "plan [prompt]",
	Short: "Generate a feature plan from a prompt without executing it",
	Long: `
Acts as a pre-flight check for 'recac start'.

This command invokes the planning agent with a given prompt to generate a
structured feature list. It prints the plan to the console and then exits,
without starting a full session, running Docker, or executing any code.

This allows you to review and validate the agent's proposed plan before
committing time and resources to a full session.
`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		prompt := strings.Join(args, " ")
		projectPath := viper.GetString("plan.path")

		// Get agent client
		provider := viper.GetString("provider")
		model := viper.GetString("model")
		projectName := filepath.Base(projectPath)

		agentClient, err := getAgentClient(ctx, provider, model, projectPath, projectName)
		if err != nil {
			return fmt.Errorf("failed to initialize agent: %w", err)
		}

		// Read app_spec.txt for additional context if it exists
		specPath := filepath.Join(projectPath, "app_spec.txt")
		specContent, err := os.ReadFile(specPath)
		var fullPrompt string
		if err == nil {
			fullPrompt = fmt.Sprintf("Existing context:\n%s\n\nNew request:\n%s", string(specContent), prompt)
			fmt.Fprintf(os.Stderr, "INFO: Using existing app_spec.txt for context.\n")
		} else {
			fullPrompt = prompt
		}

		fmt.Fprintf(os.Stderr, "INFO: Generating plan from agent...\n")

		// Generate the plan
		featureList, err := runner.GenerateFeatureList(ctx, agentClient, fullPrompt)
		if err != nil {
			return fmt.Errorf("failed to generate plan: %w", err)
		}

		// Pretty-print the result
		output, err := json.MarshalIndent(featureList, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to format plan for display: %w", err)
		}

		cmd.Println(string(output))
		return nil
	},
}
