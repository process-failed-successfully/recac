package main

import (
	"fmt"
	"os"
	"path/filepath"
	"recac/internal/policy"

	"github.com/spf13/cobra"
)

var policyFile string

var policyCmd = &cobra.Command{
	Use:   "policy",
	Short: "Manage and enforce coding policies",
	Long:  `Manage coding policies such as banned imports, file sizes, and banned content.`,
}

var policyInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a default policy file",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := ".recac"
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			if err := os.Mkdir(dir, 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		}

		path := filepath.Join(dir, "policies.yaml")
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("policy file already exists at %s", path)
		}

		content := `rules:
  - type: banned_import
    pattern: "unsafe"
    message: "Avoid using unsafe package"
  - type: file_size
    max_lines: 500
    message: "File is too large, consider refactoring"
  - type: banned_content
    pattern: "TODO: fixme"
    message: "Resolve 'TODO: fixme' before committing"
`
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write policy file: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Created default policy at %s\n", path)
		return nil
	},
}

var policyCheckCmd = &cobra.Command{
	Use:   "check [path]",
	Short: "Check the codebase against the policy",
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := policy.LoadPolicy(policyFile)
		if err != nil {
			return fmt.Errorf("failed to load policy from %s: %w", policyFile, err)
		}

		root := "."
		if len(args) > 0 {
			root = args[0]
		}

		violations, err := p.Check(root)
		if err != nil {
			return fmt.Errorf("check failed: %w", err)
		}

		if len(violations) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "✅ Policy check passed!")
			return nil
		}

		fmt.Fprintf(cmd.OutOrStdout(), "❌ Found %d policy violations:\n", len(violations))
		for _, v := range violations {
			fmt.Fprintf(cmd.OutOrStdout(), "  - %s:%d: %s (Rule: %s)\n", v.File, v.Line, v.Message, v.Rule.Type)
		}

		return fmt.Errorf("policy check failed with %d violations", len(violations))
	},
}

func init() {
	rootCmd.AddCommand(policyCmd)
	policyCmd.AddCommand(policyInitCmd)
	policyCmd.AddCommand(policyCheckCmd)
	policyCmd.PersistentFlags().StringVarP(&policyFile, "policy", "p", ".recac/policies.yaml", "Path to policy file")
}
