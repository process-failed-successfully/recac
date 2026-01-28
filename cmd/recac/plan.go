package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"recac/internal/agent/prompts"
	"recac/internal/db"
	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var planCmd = &cobra.Command{
	Use:   "plan [spec_file]",
	Short: "Generate a feature implementation plan from an application spec",
	Long:  `Analyzes the application specification (default: app_spec.txt) using the configured AI agent and generates a detailed JSON feature list.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		specFile := "app_spec.txt"
		if len(args) > 0 {
			specFile = args[0]
		}

		outputFile, _ := cmd.Flags().GetString("output")
		if outputFile == "" {
			outputFile = "feature_list.json"
		}

		// Read Spec
		content, err := os.ReadFile(specFile)
		if err != nil {
			return fmt.Errorf("failed to read spec file %s: %w", specFile, err)
		}
		specContent := string(content)

		fmt.Fprintf(cmd.OutOrStdout(), "Analyzing spec from %s...\n", specFile)

		// Prepare Agent
		ctx := context.Background()
		projectPath, _ := os.Getwd()
		projectName := filepath.Base(projectPath)

		// Use the factory for testability
		agentClient, err := agentClientFactory(ctx, viper.GetString("provider"), viper.GetString("model"), projectPath, projectName)
		if err != nil {
			return fmt.Errorf("failed to initialize agent: %w", err)
		}

		// Prepare Prompt
		prompt, err := prompts.GetPrompt(prompts.Planner, map[string]string{
			"spec": specContent,
		})
		if err != nil {
			return fmt.Errorf("failed to load planner prompt: %w", err)
		}

		// Call Agent
		fmt.Fprintln(cmd.OutOrStdout(), "Consulting Planner Agent...")
		resp, err := agentClient.Send(ctx, prompt)
		if err != nil {
			return fmt.Errorf("agent failed to generate plan: %w", err)
		}

		// Clean and Parse JSON
		jsonContent := utils.CleanJSONBlock(resp)

		var featureList db.FeatureList
		if err := json.Unmarshal([]byte(jsonContent), &featureList); err != nil {
			return fmt.Errorf("failed to parse agent response as JSON: %w\nResponse: %s", err, resp)
		}

		// Save to File
		data, err := json.MarshalIndent(featureList, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal feature list: %w", err)
		}

		if err := os.WriteFile(outputFile, data, 0644); err != nil {
			return fmt.Errorf("failed to write output file %s: %w", outputFile, err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Plan generated successfully!\n")
		fmt.Fprintf(cmd.OutOrStdout(), "Project: %s\n", featureList.ProjectName)
		fmt.Fprintf(cmd.OutOrStdout(), "Features: %d\n", len(featureList.Features))
		fmt.Fprintf(cmd.OutOrStdout(), "Saved to: %s\n", outputFile)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(planCmd)
	planCmd.Flags().StringP("output", "o", "feature_list.json", "Output file for the generated feature list")
}

func NewPlanCmd() *cobra.Command {
	return planCmd
}
