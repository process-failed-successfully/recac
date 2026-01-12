package main

import (
	"fmt"
	"recac/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(dashboardCmd)
}

var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Launch an interactive TUI dashboard for monitoring sessions",
	Long:  `Provides a real-time, consolidated dashboard view of all RECAC sessions, including their status, logs, costs, and more.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// This is where the Bubble Tea program will be started.
		// For now, it's a placeholder.
		return doDashboard(cmd)
	},
}

// doDashboard will initialize and run the Bubble Tea model.
// It is decoupled from the cobra command to facilitate testing.
func doDashboard(cmd *cobra.Command) error {
	sm, err := sessionManagerFactory()
	if err != nil {
		return fmt.Errorf("could not create session manager: %w", err)
	}

	// Inject the data loading function into the UI package
	ui.SetAgentStateLoader(loadAgentState)

	model := ui.NewDashboardModel(sm)
	p := tea.NewProgram(model)

	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}
