package main

import (
	"fmt"
	"recac/internal/analysis"
	"recac/internal/tui"

	"github.com/spf13/cobra"
)

// Mockable dependency for TUI
var startDepsFunc = tui.StartDeps

var depsCmd = &cobra.Command{
	Use:   "deps [path]",
	Short: "Interactive dependency explorer (TUI)",
	Long: `Explore the project's dependency graph interactively.
Visualize package stability, incoming (afferent) and outgoing (efferent) coupling, and identify hotspots.`,
	RunE: runDeps,
}

func init() {
	rootCmd.AddCommand(depsCmd)
}

func runDeps(cmd *cobra.Command, args []string) error {
	path := "."
	if len(args) > 0 {
		path = args[0]
	}

	// 1. Analyze Dependencies
	fmt.Fprintln(cmd.OutOrStdout(), "🔍 Analyzing dependencies...")

	moduleName, err := analysis.GetModuleName(path)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not determine module name: %v\n", err)
	}

	opts := analysis.DependencyOptions{
		Root:       path,
		ModuleName: moduleName,
		ShowStdLib: false,
	}

	deps, err := analyzeDependenciesFunc(opts)
	if err != nil {
		return fmt.Errorf("analysis failed: %w", err)
	}

	if len(deps) == 0 {
		return fmt.Errorf("no dependencies found (is this a Go module?)")
	}

	// 2. Launch TUI
	// Convert analysis.DepMap to map[string][]string
	depsMap := make(map[string][]string)
	for k, v := range deps {
		depsMap[k] = v
	}

	if err := startDepsFunc(depsMap); err != nil {
		return fmt.Errorf("tui failed: %w", err)
	}

	return nil
}
