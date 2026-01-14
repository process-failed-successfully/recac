package main

import (
	"recac/internal/model"
	"recac/internal/ui"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(topCmd)
}

var topCmd = &cobra.Command{
	Use:   "top",
	Short: "Monitor live session resource usage",
	Long:  `Display a live dashboard of running sessions and their real-time CPU and memory consumption.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ui.GetTopSessions = func() ([]model.UnifiedSession, error) {
			return getRunningSessions(cmd)
		}
		return ui.StartTopDashboard()
	},
}
