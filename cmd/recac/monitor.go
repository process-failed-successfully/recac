package main

import (
	"recac/internal/tui"

	"github.com/spf13/cobra"
)

var (
	monitorHost string
	// For testing purposes
	startDashboard = tui.StartDashboard
)

func init() {
	monitorCmd.Flags().StringVar(&monitorHost, "host", "http://localhost:2112", "Orchestrator host URL")
	rootCmd.AddCommand(monitorCmd)
}

var monitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Monitor the RECAC Orchestrator",
	Long:  `Launch the TUI dashboard to monitor active jobs and orchestrator status in real-time.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return startDashboard(monitorHost)
	},
}
