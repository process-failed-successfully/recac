package main

import (
	"fmt"
	"recac/internal/tui"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var startDashboard = tui.StartDashboard

var monitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Launch the orchestrator dashboard",
	Long:  `Launch the TUI dashboard to monitor the orchestrator status and jobs.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var host string
		if cmd.Flags().Changed("host") {
			host, _ = cmd.Flags().GetString("host")
		} else {
			host = viper.GetString("monitor.host")
		}

		if host == "" {
			host = "http://localhost:2112"
		}

		if err := startDashboard(host); err != nil {
			return fmt.Errorf("dashboard failed: %w", err)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(monitorCmd)

	monitorCmd.Flags().String("host", "http://localhost:2112", "Orchestrator host URL")
	viper.BindPFlag("monitor.host", monitorCmd.Flags().Lookup("host"))
}
