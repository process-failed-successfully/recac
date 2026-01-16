package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"recac/internal/agent/prompts"
	"recac/internal/db"
	"recac/internal/ui"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	planCmd.Flags().String("path", "", "Project path (defaults to current directory)")
	viper.BindPFlag("plan.path", planCmd.Flags().Lookup("path"))
	rootCmd.AddCommand(planCmd)
}

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Generate a development plan from app_spec.txt",
	Long: `Analyze the app_spec.txt file using the configured AI agent and generate a detailed feature list (plan).
You can review the plan and save it to feature_list.json, which is used by 'recac start'.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		// 1. Determine Path
		projectPath := viper.GetString("plan.path")
		if projectPath == "" {
			// Fallback to flag directly in case Viper binding fails (common in tests)
			projectPath, _ = cmd.Flags().GetString("path")
		}
		if projectPath == "" {
			var err error
			projectPath, err = os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current working directory: %w", err)
			}
		}

		cmd.Printf("Planning for project in: %s\n", projectPath)

		// 2. Read app_spec.txt
		specPath := filepath.Join(projectPath, "app_spec.txt")
		specContent, err := os.ReadFile(specPath)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("app_spec.txt not found in %s. Please create it first", projectPath)
			}
			return fmt.Errorf("failed to read app_spec.txt: %w", err)
		}

		if len(specContent) == 0 {
			return fmt.Errorf("app_spec.txt is empty")
		}

		// 3. Initialize Agent
		provider := viper.GetString("provider")
		model := viper.GetString("model")
		projectName := filepath.Base(projectPath)

		cmd.Printf("Initializing Agent (%s/%s)...\n", provider, model)
		agentClient, err := agentClientFactory(ctx, provider, model, projectPath, projectName)
		if err != nil {
			return fmt.Errorf("failed to initialize agent: %w", err)
		}

		// 4. Generate Prompt
		cmd.Println("Analyzing specification...")
		prompt, err := prompts.GetPrompt(prompts.Planner, map[string]string{
			"spec": string(specContent),
		})
		if err != nil {
			return fmt.Errorf("failed to load planner prompt: %w", err)
		}

		// 5. Send to Agent
		cmd.Println("Generating plan (this may take a minute)...")
		response, err := agentClient.Send(ctx, prompt)
		if err != nil {
			return fmt.Errorf("agent request failed: %w", err)
		}

		// 6. Parse JSON
		// Clean markdown code blocks if present
		jsonContent := cleanJSON(response)
		var featureList db.FeatureList
		if err := json.Unmarshal([]byte(jsonContent), &featureList); err != nil {
			cmd.Println("--- Raw Agent Response ---")
			cmd.Println(response)
			cmd.Println("--------------------------")
			return fmt.Errorf("failed to parse agent response as JSON: %w", err)
		}

		// 7. Display Plan
		cmd.Println("\n=== 📋 Generated Plan ===")
		cmd.Printf("Project: %s\n", featureList.ProjectName)
		cmd.Printf("Total Features: %d\n\n", len(featureList.Features))

		for i, f := range featureList.Features {
			status := "Pending"
			if f.Status != "" {
				status = f.Status
			}
			// Use RenderMarkdown for nice description if possible, or just print
			desc := ui.RenderMarkdown(f.Description, 80)
			cmd.Printf("%d. [%s] (%s) %s\n", i+1, f.ID, f.Priority, status)
			cmd.Printf("   %s\n", strings.ReplaceAll(desc, "\n", "\n   "))

			if len(f.Dependencies.DependsOnIDs) > 0 {
				cmd.Printf("   Depends on: %v\n", f.Dependencies.DependsOnIDs)
			}
			cmd.Println()
		}

		// 8. Confirm Save
		confirm := false
		promptSave := &survey.Confirm{
			Message: "Save this plan to feature_list.json?",
			Default: true,
		}
		if err := surveyAskOne(promptSave, &confirm); err != nil {
			return fmt.Errorf("prompt failed: %w", err)
		}

		if confirm {
			outputPath := filepath.Join(projectPath, "feature_list.json")
			if err := os.WriteFile(outputPath, []byte(jsonContent), 0644); err != nil {
				return fmt.Errorf("failed to save feature_list.json: %w", err)
			}
			cmd.Printf("✅ Plan saved to %s\n", outputPath)
			cmd.Println("Run 'recac start' to begin execution!")
		} else {
			cmd.Println("❌ Plan discarded.")
		}

		return nil
	},
}

func cleanJSON(s string) string {
	s = strings.TrimSpace(s)
	// Remove markdown code blocks
	if strings.HasPrefix(s, "```json") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimSuffix(s, "```")
	} else if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimSuffix(s, "```")
	}
	return strings.TrimSpace(s)
}
