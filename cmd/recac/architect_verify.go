package main

import (
	"fmt"
	"os"

	"recac/internal/analysis"
	"recac/internal/architecture"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var architectVerifyCmd = &cobra.Command{
	Use:   "verify [path-to-architecture.yaml]",
	Short: "Verify code compliance with architecture",
	Long: `Analyzes the codebase dependencies and verifies they comply with the defined architecture.
Checks if components import other components that are not explicitly declared as dependencies (Consumes).`,
	RunE: runArchitectVerify,
}

func init() {
	architectCmd.AddCommand(architectVerifyCmd)
}

func runArchitectVerify(cmd *cobra.Command, args []string) error {
	path := ".recac/architecture/architecture.yaml"
	if len(args) > 0 {
		path = args[0]
	}

	// 1. Read Architecture
	fmt.Fprintf(cmd.OutOrStdout(), "📖 Reading architecture from %s...\n", path)
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read architecture file: %w", err)
	}

	var arch architecture.SystemArchitecture
	if err := yaml.Unmarshal(data, &arch); err != nil {
		return fmt.Errorf("failed to parse architecture YAML: %w", err)
	}

	// 2. Analyze Dependencies
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), "🔍 Analyzing code dependencies...")

	moduleName, err := analysis.GetModuleName(cwd)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not read go.mod: %v\n", err)
		moduleName = ""
	}

	opts := analysis.DependencyOptions{
		Root:       cwd,
		ModuleName: moduleName,
		ShowStdLib: false,
	}
	deps, err := analysis.AnalyzeDependencies(opts)
	if err != nil {
		return fmt.Errorf("dependency analysis failed: %w", err)
	}

	// 3. Verify
	fmt.Fprintln(cmd.OutOrStdout(), "⚖️  Verifying compliance...")
	verifier := architecture.NewVerifier(&arch)
	violations, err := verifier.Verify(deps)
	if err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	if len(violations) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "✅ Architecture verification passed! No violations found.")
		return nil
	}

	// 4. Report Violations
	fmt.Fprintf(cmd.OutOrStdout(), "❌ Found %d architectural violations:\n", len(violations))
	for _, v := range violations {
		fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", v)
	}

	return fmt.Errorf("architecture verification failed")
}
