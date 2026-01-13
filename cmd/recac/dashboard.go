package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"recac/internal/ui"
)

var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Launch a real-time TUI dashboard",
	Long:  `The dashboard provides a live overview of sessions, system status, and costs.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %w", err)
		}

		if err := ui.StartDashboard(sm); err != nil {
			// TUI errors are often not useful to print to stdout
			// as they can mess up the terminal. The TUI program
			// itself usually handles printing the error.
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(dashboardCmd)
}
