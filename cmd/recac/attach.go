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

		// We allow attaching to completed/stopped sessions to view logs
		if session.Status != "running" {
			fmt.Printf("Note: Session '%s' is %s (not running). Showing logs only.\n", sessionName, session.Status)
		}

		logFile, err := sm.GetSessionLogs(sessionName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting logs: %v\n", err)
			exit(1)
		}

		// Start TUI Dashboard
		fetchSession := func() (*runner.SessionState, error) {
			return sm.LoadSession(sessionName)
		}

		if err := ui.StartAttachDashboard(sessionName, logFile, fetchSession); err != nil {
			fmt.Fprintf(os.Stderr, "Error running dashboard: %v\n", err)
			exit(1)
		}
	},
}
