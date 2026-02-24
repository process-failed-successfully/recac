package main

import (
	"context"
	"fmt"
	"os"
	"recac/internal/tui"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Start an interactive AI pair programming session (TUI)",
	Long: `Launches a terminal user interface for interactive coding assistance.
This provides a better experience than 'chat' with history scrolling,
markdown rendering, and multi-line input support.`,
	RunE: runSession,
}

func runSession(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	cwd, _ := os.Getwd()
	provider := viper.GetString("provider")
	model := viper.GetString("model")

	// Use factory from factories.go (available in package main)
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-session")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	// Start TUI
	if err := tui.StartSession(ag); err != nil {
		return fmt.Errorf("session failed: %w", err)
	}

	return nil
}

func init() {
	rootCmd.AddCommand(sessionCmd)
}
