package main

import (
	"fmt"
	"recac/internal/ui"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(statusCmd)
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the status of RECAC sessions and environment",
	Long:  `Displays a summary of all running and completed RECAC sessions, checks the Docker environment, and shows key configuration values.`,
	Run: func(cmd *cobra.Command, args []string) {
		// All logic is now centralized in ui.GetStatus()
		// to ensure consistency between the CLI and TUI.
		fmt.Print(ui.GetStatus())
	},
}
