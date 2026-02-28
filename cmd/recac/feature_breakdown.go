package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var featureBreakdownCmd = &cobra.Command{
	Use:   "breakdown <description>",
	Short: "Break down a feature into tasks and append them to TODO.md",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		featureDesc := args[0]

		fmt.Fprintf(cmd.OutOrStdout(), "Breaking down feature: %s\n", featureDesc)

		ctx := context.Background()
		projectPath, _ := os.Getwd()
		projectName := filepath.Base(projectPath)

		ag, err := agentClientFactory(ctx, viper.GetString("provider"), viper.GetString("model"), projectPath, projectName)
		if err != nil {
			return fmt.Errorf("failed to initialize agent: %w", err)
		}

		promptStr := fmt.Sprintf(`Break down the following feature into small, actionable tasks: "%s"

Respond ONLY with a JSON array of strings, where each string is a specific task.
Example: ["Update database schema", "Create API endpoint", "Write unit tests"]
`, featureDesc)

		fmt.Fprintln(cmd.OutOrStdout(), "Consulting AI agent to generate tasks...")
		resp, err := ag.Send(ctx, promptStr)
		if err != nil {
			return fmt.Errorf("agent failed to generate tasks: %w", err)
		}

		jsonContent := utils.CleanCodeBlock(resp)

		var tasks []string
		if err := json.Unmarshal([]byte(jsonContent), &tasks); err != nil {
			return fmt.Errorf("failed to parse agent response as JSON: %w\nResponse: %s", err, resp)
		}

		if len(tasks) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "AI agent did not generate any tasks.")
			return nil
		}

		todoFile := "TODO.md"
		f, err := os.OpenFile(todoFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("failed to open %s: %w", todoFile, err)
		}
		defer f.Close()

		// If file is empty, write a header
		info, err := os.Stat(todoFile)
		if err == nil && info.Size() == 0 {
			if _, err := f.WriteString("# TODO\n\n"); err != nil {
				return err
			}
		} else {
			// Ensure we are on a new line
			if _, err := f.WriteString("\n"); err != nil {
				return err
			}
		}

		for _, task := range tasks {
			if _, err := f.WriteString(fmt.Sprintf("- [ ] %s\n", task)); err != nil {
				return err
			}
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Successfully appended %d tasks to %s\n", len(tasks), todoFile)
		return nil
	},
}
