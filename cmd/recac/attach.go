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

		// Use the factory if available (for tests), otherwise default
		var sm runner.ISessionManager
		var err error
		if sessionManagerFactory != nil {
			sm, err = sessionManagerFactory()
		} else {
			sm, err = runner.NewSessionManager()
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to create session manager: %v\n", err)
			exit(1)
		}

		session, err := sm.LoadSession(sessionName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: session not found: %v\n", err)
			exit(1)
		}

		// Allow viewing logs of non-running sessions too, but print status
		if session.Status != "running" {
			fmt.Printf("Note: session '%s' is currently %s\n", sessionName, session.Status)
		}

		if err := ui.StartAttachDashboard(sessionName, sm); err != nil {
			fmt.Fprintf(os.Stderr, "Error attaching to session: %v\n", err)
			exit(1)
		}
	},
}
