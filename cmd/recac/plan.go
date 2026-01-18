package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"recac/internal/agent/prompts"
	"recac/internal/db"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	rootCmd.AddCommand(NewPlanCmd())
}

func NewPlanCmd() *cobra.Command {
	cmd := &cobra.Command{
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
			jsonContent := cleanJSON(resp)

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

	cmd.Flags().StringP("output", "o", "feature_list.json", "Output file for the generated feature list")
	return cmd
}

// cleanJSON strips markdown code blocks if present
func cleanJSON(content string) string {
	content = strings.TrimSpace(content)

	// Try to find markdown code blocks
	start := strings.Index(content, "```")
	if start != -1 {
		// Found a code block start
		// Check if it's ```json or just ```
		codeStart := start + 3
		if strings.HasPrefix(content[codeStart:], "json") {
			codeStart += 4
		}

		// Find the end of the block
		end := strings.Index(content[codeStart:], "```")
		if end != -1 {
			// Extract the content inside the block
			return strings.TrimSpace(content[codeStart : codeStart+end])
		}
	}

	// Fallback: If no code blocks, look for the first '{' and last '}'
	firstBrace := strings.Index(content, "{")
	lastBrace := strings.LastIndex(content, "}")
	if firstBrace != -1 && lastBrace != -1 && lastBrace > firstBrace {
		return strings.TrimSpace(content[firstBrace : lastBrace+1])
	}

	return content
}
