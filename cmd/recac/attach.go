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
			// Instead of exiting, we warn the user but allow viewing logs
			fmt.Printf("Warning: session '%s' is not running (status: %s)\n", sessionName, session.Status)
			fmt.Println("Starting dashboard in read-only mode (press q to exit)...")
		}

		// Inject dependency
		ui.GetSession = sm.LoadSession

		if err := ui.StartAttachDashboard(sessionName); err != nil {
			fmt.Fprintf(os.Stderr, "Error running dashboard: %v\n", err)
			exit(1)
		}
	},
}
