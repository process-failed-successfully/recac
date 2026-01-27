package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"recac/internal/agent/prompts"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	ptVars     []string
	ptJsonFile string
	ptDryRun   bool
	ptModel    string
	ptSaveFile string
)

func init() {
	promptTestCmd.Flags().StringArrayVarP(&ptVars, "var", "V", nil, "Set template variables (key=value)")
	promptTestCmd.Flags().StringVarP(&ptJsonFile, "json-file", "j", "", "Load variables from a JSON file")
	promptTestCmd.Flags().BoolVarP(&ptDryRun, "dry-run", "d", false, "Render prompt without calling AI")
	promptTestCmd.Flags().StringVarP(&ptModel, "model", "m", "", "Override AI model")
	promptTestCmd.Flags().StringVarP(&ptSaveFile, "save", "s", "", "Save response to file")

	promptCmd.AddCommand(promptTestCmd)
}

var promptTestCmd = &cobra.Command{
	Use:   "test [prompt_name]",
	Short: "Test a prompt template interactively",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		vars := make(map[string]string)

		// 1. Load variables from JSON file
		if ptJsonFile != "" {
			content, err := os.ReadFile(ptJsonFile)
			if err != nil {
				return fmt.Errorf("failed to read JSON file: %w", err)
			}
			if err := json.Unmarshal(content, &vars); err != nil {
				return fmt.Errorf("failed to parse JSON file: %w", err)
			}
		}

		// 2. Load variables from flags (override JSON)
		for _, v := range ptVars {
			parts := strings.SplitN(v, "=", 2)
			if len(parts) != 2 {
				return fmt.Errorf("invalid variable format: %s (expected key=value)", v)
			}
			vars[parts[0]] = parts[1]
		}

		// 3. Load and render prompt
		promptContent, err := prompts.GetPrompt(name, vars)
		if err != nil {
			return err
		}

		// 4. Handle Dry Run
		if ptDryRun {
			fmt.Fprintln(cmd.OutOrStdout(), promptContent)
			return nil
		}

		// 5. Initialize Agent
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		provider := viper.GetString("provider")
		model := viper.GetString("model")
		if ptModel != "" {
			model = ptModel
		}

		cwd, _ := os.Getwd()
		agent, err := agentClientFactory(ctx, provider, model, cwd, "recac-prompt-test")
		if err != nil {
			return fmt.Errorf("failed to create agent: %w", err)
		}

		// 6. Send to Agent
		fmt.Fprintln(cmd.OutOrStdout(), "Sending prompt to AI...")
		var responseBuilder strings.Builder
		response, err := agent.SendStream(ctx, promptContent, func(chunk string) {
			fmt.Fprint(cmd.OutOrStdout(), chunk)
			responseBuilder.WriteString(chunk)
		})
		fmt.Fprintln(cmd.OutOrStdout(), "") // Ensure newline at end

		if err != nil {
			return fmt.Errorf("agent error: %w", err)
		}

		// 7. Save response
		if ptSaveFile != "" {
			// Use the full accumulated response from SendStream return value if available,
			// or the builder if not (SendStream returns full string).
			if response == "" {
				response = responseBuilder.String()
			}
			if err := os.WriteFile(ptSaveFile, []byte(response), 0644); err != nil {
				return fmt.Errorf("failed to save response to %s: %w", ptSaveFile, err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Response saved to %s\n", ptSaveFile)
		}

		return nil
	},
}
