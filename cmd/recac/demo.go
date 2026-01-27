package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"recac/internal/utils"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	demoOutput string
	demoRender bool
	demoFormat string
	demoAuto   bool
)

var demoCmd = &cobra.Command{
	Use:   "demo [scenario]",
	Short: "Generate an automated terminal demo (VHS tape) using AI",
	Long: `Generates a VHS tape script (.tape file) for a specified scenario using the AI agent.
If 'vhs' is installed, it can automatically render the demo to a GIF or MP4.

Example:
  recac demo "Show how to use the investigate command"
  recac demo "Demonstrate the heal command failure and recovery" --format mp4
`,
	Args: cobra.MinimumNArgs(1),
	RunE: runDemo,
}

func init() {
	rootCmd.AddCommand(demoCmd)
	demoCmd.Flags().StringVarP(&demoOutput, "output", "o", "demo.tape", "Output tape file path")
	demoCmd.Flags().BoolVarP(&demoRender, "render", "r", true, "Render the tape using vhs (if installed)")
	demoCmd.Flags().StringVarP(&demoFormat, "format", "f", "gif", "Output format (gif, mp4, webm)")
	demoCmd.Flags().BoolVar(&demoAuto, "auto", false, "Automatically run vhs without confirmation")
}

func runDemo(cmd *cobra.Command, args []string) error {
	scenario := strings.Join(args, " ")
	ctx := cmd.Context()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	// 1. Prepare Agent
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-demo")
	if err != nil {
		return fmt.Errorf("failed to initialize agent: %w", err)
	}

	// 2. Construct Prompt
	prompt := fmt.Sprintf(`You are an expert at creating VHS tape scripts for terminal demos.
Create a VHS tape script to demonstrate the following scenario using the 'recac' CLI.

Scenario: "%s"

The script should be a valid .tape file.
- Use 'Output <filename>.%s'
- Set FontSize 20
- Set Width 1200
- Set Height 600
- Use 'Type' to type commands.
- Use 'Enter' to execute.
- Use 'Sleep' for timing.
- Assume the 'recac' binary is in the PATH or use './bin/recac'.

Return ONLY the content of the .tape file. Do not use Markdown code blocks.`, scenario, demoFormat)

	fmt.Fprintf(cmd.OutOrStdout(), "🎬 Generating demo script for: %s...\n", scenario)

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed to generate script: %w", err)
	}

	// 3. Clean Script
	tapeContent := utils.CleanCodeBlock(resp)

	// 4. Save Script First
	if err := mkdirAllFunc(filepath.Dir(demoOutput), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := writeFileFunc(demoOutput, []byte(tapeContent), 0644); err != nil {
		return fmt.Errorf("failed to write tape file: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✅ Script saved to %s\n", demoOutput)

	// 5. Render (Optional)
	if demoRender {
		// Check if vhs is installed
		if _, err := execCommand("vhs", "--version").Output(); err != nil {
			fmt.Fprintln(cmd.ErrOrStderr(), "⚠️  'vhs' tool not found. Skipping render. Install it with 'go install github.com/charmbracelet/vhs@latest'")
			return nil
		}

		// Confirmation
		if !demoAuto {
			fmt.Fprintln(cmd.OutOrStdout(), "\n--- Generated Script ---")
			fmt.Fprintln(cmd.OutOrStdout(), tapeContent)
			fmt.Fprintln(cmd.OutOrStdout(), "------------------------")

			confirm := false
			err := askOneFunc(&survey.Confirm{
				Message: "Execute this script with vhs?",
				Default: false,
			}, &confirm)
			if err != nil {
				return err // User cancelled
			}

			if !confirm {
				fmt.Fprintln(cmd.OutOrStdout(), "Skipping execution.")
				return nil
			}
		}

		fmt.Fprintln(cmd.OutOrStdout(), "🎥 Rendering video (this may take a while)...")
		renderCmd := execCommand("vhs", demoOutput)
		renderCmd.Stdout = cmd.OutOrStdout()
		renderCmd.Stderr = cmd.ErrOrStderr()

		if err := renderCmd.Run(); err != nil {
			return fmt.Errorf("rendering failed: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "✨ Render complete!")
	}

	return nil
}
