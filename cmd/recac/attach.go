package main

import (
	"fmt"
	"os"
	"recac/internal/ui"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(attachCmd)
}

var attachCmd = &cobra.Command{
	Use:   "attach [session-name]",
	Short: "Re-attach to a session and view logs",
	Long:  `Re-attach to a running or completed session to view its output in real-time.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		sessionName := args[0]

		if err := ui.StartAttachDashboard(sessionName); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			exit(1)
		}
	},
}
