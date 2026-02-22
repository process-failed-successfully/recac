package main

import (
	"fmt"
	"os"

	"recac/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var monitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Launch the interactive TUI dashboard",
	Long:  `Launch a real-time terminal user interface (TUI) to monitor active agent sessions, view logs, and track status.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Create Session Manager
		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %w", err)
		}

		// Initialize TUI Model
		model := tui.NewLocalDashboardModel(sm)

		// Run Program
		p := tea.NewProgram(model, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			fmt.Printf("Error running dashboard: %v\n", err)
			os.Exit(1)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(monitorCmd)
}
