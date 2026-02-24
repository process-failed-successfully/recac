package main

import (
	"fmt"
	"os"
	"recac/internal/tui"

	"github.com/spf13/cobra"
)

var explorerCmd = &cobra.Command{
	Use:   "explorer [path]",
	Short: "Interactive file system explorer (TUI)",
	Long: `Explore your codebase with an interactive terminal UI.
Navigate files, preview content, and view metadata.`,
	RunE: runExplorer,
}

func init() {
	rootCmd.AddCommand(explorerCmd)
}

func runExplorer(cmd *cobra.Command, args []string) error {
	path := "."
	if len(args) > 0 {
		path = args[0]
	}

	// Verify path exists
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("invalid path %s: %w", path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path %s is not a directory", path)
	}

	// Launch TUI
	if err := tui.StartExplorer(path); err != nil {
		return fmt.Errorf("explorer failed: %w", err)
	}

	return nil
}
