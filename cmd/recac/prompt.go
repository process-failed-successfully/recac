package main

import (
	"fmt"
	"os"
	"path/filepath"
	"recac/internal/agent/prompts"

	"github.com/spf13/cobra"
)

var (
	promptOverrideOut string
	promptForce       bool
)

func init() {
	rootCmd.AddCommand(promptCmd)
	promptCmd.AddCommand(promptListCmd)
	promptCmd.AddCommand(promptShowCmd)
	promptCmd.AddCommand(promptOverrideCmd)

	promptOverrideCmd.Flags().StringVarP(&promptOverrideOut, "out", "o", "", "Output directory or file path")
	promptOverrideCmd.Flags().BoolVarP(&promptForce, "force", "f", false, "Overwrite existing file")
}

var promptCmd = &cobra.Command{
	Use:   "prompt",
	Short: "Manage and inspect AI agent prompts",
	Long:  `List, view, and override the system prompts used by the AI agents.`,
}

var promptListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available prompt templates",
	RunE: func(cmd *cobra.Command, args []string) error {
		names, err := prompts.ListPrompts()
		if err != nil {
			return err
		}
		for _, name := range names {
			fmt.Fprintln(cmd.OutOrStdout(), name)
		}
		return nil
	},
}

var promptShowCmd = &cobra.Command{
	Use:   "show [name]",
	Short: "Show the content of a prompt template",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		content, err := prompts.GetPrompt(name, nil)
		if err != nil {
			return err
		}
		fmt.Fprint(cmd.OutOrStdout(), content)
		return nil
	},
}

var promptOverrideCmd = &cobra.Command{
	Use:   "override [name]",
	Short: "Extract a prompt template to a local file for customization",
	Long: `Extracts the specified prompt template to a local file.
If --out is not specified, it defaults to the current directory with the name <name>.md.
You can then edit this file and set RECAC_PROMPTS_DIR to the directory containing it to make RECAC use your custom prompt.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		content, err := prompts.GetPrompt(name, nil)
		if err != nil {
			return err
		}

		// Determine output path
		var outPath string
		if promptOverrideOut != "" {
			info, err := os.Stat(promptOverrideOut)
			if err == nil && info.IsDir() {
				outPath = filepath.Join(promptOverrideOut, name+".md")
			} else {
				outPath = promptOverrideOut
			}
		} else {
			// Default to current directory
			outPath = name + ".md"
		}

		// Check existence
		if _, err := os.Stat(outPath); err == nil && !promptForce {
			return fmt.Errorf("file %s already exists; use --force to overwrite", outPath)
		}

		if err := os.WriteFile(outPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Prompt '%s' saved to %s\n", name, outPath)

		// Check if RECAC_PROMPTS_DIR is set and relevant
		promptsDir := os.Getenv("RECAC_PROMPTS_DIR")
		absDir, _ := filepath.Abs(filepath.Dir(outPath))

		if promptsDir == "" {
			fmt.Fprintf(cmd.OutOrStdout(), "\nTo use this prompt, set:\nexport RECAC_PROMPTS_DIR=\"%s\"\n", absDir)
		} else {
			absPromptsDir, _ := filepath.Abs(promptsDir)
			if absPromptsDir != absDir {
				fmt.Fprintf(cmd.OutOrStdout(), "\nWarning: RECAC_PROMPTS_DIR is set to \"%s\", but you saved the prompt to \"%s\".\nIt might not be picked up.\n", promptsDir, absDir)
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "\nRECAC_PROMPTS_DIR is correctly set. Your override should be active.")
			}
		}

		return nil
	},
}
