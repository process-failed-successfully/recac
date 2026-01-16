package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(restartCmd)
}

var restartCmd = &cobra.Command{
	Use:   "restart [session-name]",
	Short: "Restart a session",
	Long:  `Restarts a session by using its original command and workspace. This is useful for re-running failed or completed sessions.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %w", err)
		}

		var sessionName string
		if len(args) == 0 {
			sessionName, err = interactiveSelectRestartableSession(sm, "Choose a session to restart:")
			if err != nil {
				return err
			}
		} else {
			sessionName = args[0]
		}

		// Load the session to get its details
		session, err := sm.LoadSession(sessionName)
		if err != nil {
			return fmt.Errorf("failed to load session '%s': %w", sessionName, err)
		}

		// Check if the session is running
		if sm.IsProcessRunning(session.PID) {
			return fmt.Errorf("cannot restart a running session: '%s' (PID: %d)", sessionName, session.PID)
		}

		// Remove the old session files to ensure a clean restart.
		// Use force=true because we have already confirmed the process is not running.
		if err := sm.RemoveSession(sessionName, true); err != nil {
			return fmt.Errorf("failed to remove old session files for '%s': %w", sessionName, err)
		}

		// Start a new session with the same parameters
		newSession, err := sm.StartSession(session.Name, session.Command, session.Workspace)
		if err != nil {
			return fmt.Errorf("failed to restart session '%s': %w", session.Name, err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Session '%s' restarted successfully (PID: %d)\n", newSession.Name, newSession.PID)
		return nil
	},
}
