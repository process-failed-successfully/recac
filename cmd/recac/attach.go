package main

import (
	"fmt"
	"os"
	"recac/internal/runner"
	"recac/internal/ui"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(attachCmd)
}

var attachCmd = &cobra.Command{
	Use:   "attach [session-name]",
	Short: "Re-attach to a running session",
	Long:  `Re-attach to a running session to view its output in real-time.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		sessionName := args[0]

		sm, err := runner.NewSessionManager()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to create session manager: %v\n", err)
			exit(1)
		}

		session, err := sm.LoadSession(sessionName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: session not found: %v\n", err)
			exit(1)
		}

		if session.Status != "running" {
			fmt.Fprintf(os.Stderr, "Error: session '%s' is not running (status: %s)\n", sessionName, session.Status)
			exit(1)
		}

		// Stream logs
		logFile, err := sm.GetSessionLogs(sessionName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			exit(1)
		}

		if err := ui.StartAttachDashboard(sessionName, logFile); err != nil {
			fmt.Fprintf(os.Stderr, "Error showing dashboard: %v\n", err)
			exit(1)
		}
	},
}
