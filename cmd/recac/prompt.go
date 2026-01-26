package main

import (
	"fmt"
	"os"
	"path/filepath"
	"recac/internal/agent/prompts"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var (
	promptGlobal bool
)

func init() {
	rootCmd.AddCommand(promptCmd)
	promptCmd.AddCommand(promptListCmd)
	promptCmd.AddCommand(promptShowCmd)
	promptCmd.AddCommand(promptOverrideCmd)
	promptCmd.AddCommand(promptResetCmd)

	promptOverrideCmd.Flags().BoolVarP(&promptGlobal, "global", "g", false, "Override globally (~/.recac/prompts)")
	promptResetCmd.Flags().BoolVarP(&promptGlobal, "global", "g", false, "Reset global override")
}

var promptCmd = &cobra.Command{
	Use:   "prompt",
	Short: "Manage AI system prompts",
	Long:  `List, view, and customize the system prompts used by the AI agents.`,
}

var promptListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available prompts",
	RunE: func(cmd *cobra.Command, args []string) error {
		names, err := prompts.ListPrompts()
		if err != nil {
			return err
		}

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "NAME\tSOURCE")

		for _, name := range names {
			source := "Embedded"

			// Check overrides logic
			if hasEnvOverride(name) {
				source = "Env ($RECAC_PROMPTS_DIR)"
			} else if hasLocalOverride(name) {
				source = "Local (.recac/prompts)"
			} else if hasGlobalOverride(name) {
				source = "Global (~/.recac/prompts)"
			}

			fmt.Fprintf(w, "%s\t%s\n", name, source)
		}
		w.Flush()
		return nil
	},
}

var promptShowCmd = &cobra.Command{
	Use:   "show [name]",
	Short: "Show prompt content",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		content, err := prompts.GetPrompt(name, nil)
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), content)
		return nil
	},
}

var promptOverrideCmd = &cobra.Command{
	Use:   "override [name]",
	Short: "Override a prompt locally or globally",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		content, err := prompts.GetPrompt(name, nil)
		if err != nil {
			return err
		}

		targetDir := ".recac/prompts"
		if promptGlobal {
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			targetDir = filepath.Join(home, ".recac", "prompts")
		}

		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", targetDir, err)
		}

		targetPath := filepath.Join(targetDir, name+".md")
		if err := os.WriteFile(targetPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write override to %s: %w", targetPath, err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Prompt '%s' saved to %s\n", name, targetPath)
		return nil
	},
}

var promptResetCmd = &cobra.Command{
	Use:   "reset [name]",
	Short: "Remove override for a prompt",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		targetDir := ".recac/prompts"
		if promptGlobal {
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			targetDir = filepath.Join(home, ".recac", "prompts")
		}

		targetPath := filepath.Join(targetDir, name+".md")
		if _, err := os.Stat(targetPath); os.IsNotExist(err) {
			return fmt.Errorf("override not found at %s", targetPath)
		}

		if err := os.Remove(targetPath); err != nil {
			return fmt.Errorf("failed to remove %s: %w", targetPath, err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Override removed: %s\n", targetPath)
		return nil
	},
}

// Helpers for list command
func hasEnvOverride(name string) bool {
	overrideDir := os.Getenv("RECAC_PROMPTS_DIR")
	if overrideDir == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(overrideDir, name+".md"))
	return err == nil
}

func hasLocalOverride(name string) bool {
	cwd, err := os.Getwd()
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(cwd, ".recac", "prompts", name+".md"))
	return err == nil
}

func hasGlobalOverride(name string) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(home, ".recac", "prompts", name+".md"))
	return err == nil
}
