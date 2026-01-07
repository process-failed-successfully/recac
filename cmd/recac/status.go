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
		// Since ui.GetStatus() handles errors internally by printing them,
		// we no longer need to check for an error here. The command will
		// always exit with 0, providing the user with as much status
		// information as possible.
		showStatus()
	},
}

// showStatus prints the RECAC status by calling the shared UI function.
func showStatus() {
	fmt.Print(ui.GetStatus())
}
