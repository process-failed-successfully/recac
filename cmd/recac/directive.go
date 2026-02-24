package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var directiveCmd = &cobra.Command{
	Use:   "directive",
	Short: "Manage global project directives for the AI agent",
	Long: `Set, show, or clear the global directive (instruction) that is prepended to every AI agent prompt.
This is useful for enforcing project-wide rules like "Use TypeScript", "Prefer functional programming", or "Do not remove comments".
The directive is stored in .recac/directive file in the current working directory.`,
}

var directiveSetCmd = &cobra.Command{
	Use:   "set [instruction]",
	Short: "Set the global directive",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		instruction := strings.Join(args, " ")
		return setDirective(instruction)
	},
}

var directiveShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show the current global directive",
	RunE: func(cmd *cobra.Command, args []string) error {
		return showDirective(cmd)
	},
}

var directiveClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear the global directive",
	RunE: func(cmd *cobra.Command, args []string) error {
		return clearDirective()
	},
}

func init() {
	rootCmd.AddCommand(directiveCmd)
	directiveCmd.AddCommand(directiveSetCmd)
	directiveCmd.AddCommand(directiveShowCmd)
	directiveCmd.AddCommand(directiveClearCmd)
}

func getDirectivePath() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(cwd, ".recac", "directive"), nil
}

func setDirective(instruction string) error {
	path, err := getDirectivePath()
	if err != nil {
		return err
	}

	// Ensure .recac dir exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	if err := os.WriteFile(path, []byte(instruction), 0644); err != nil {
		return fmt.Errorf("failed to write directive: %w", err)
	}

	fmt.Printf("✅ Directive set: \"%s\"\n", instruction)
	return nil
}

func showDirective(cmd *cobra.Command) error {
	path, err := getDirectivePath()
	if err != nil {
		return err
	}

	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No global directive set.")
			return nil
		}
		return fmt.Errorf("failed to read directive: %w", err)
	}

	fmt.Printf("Global Directive:\n%s\n", string(content))
	return nil
}

func clearDirective() error {
	path, err := getDirectivePath()
	if err != nil {
		return err
	}

	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No global directive to clear.")
			return nil
		}
		return fmt.Errorf("failed to clear directive: %w", err)
	}

	fmt.Println("✅ Directive cleared.")
	return nil
}
