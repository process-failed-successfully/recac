package main

import (
	"fmt"
	"os"
	"recac/internal/runner"
	"time"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(replayCmd)
}

var replayCmd = &cobra.Command{
	Use:   "replay [session-name]",
	Short: "Replay a previous session",
	Long:  `Replay a previous session by starting a new one with the same command and workspace.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionName := args[0]

		sm, err := runner.NewSessionManager()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %w", err)
		}

		// Load the original session
		originalSession, err := sm.LoadSession(sessionName)
		if err != nil {
			return fmt.Errorf("failed to load session '%s': %w", sessionName, err)
		}

		// Prevent replaying a running session to avoid unexpected behavior
		if originalSession.Status == "running" && sm.IsProcessRunning(originalSession.PID) {
			return fmt.Errorf("cannot replay a running session. Please stop it first")
		}

		// Create a new name for the replayed session
		var replayName string
		if os.Getenv("RECAC_TEST") == "1" {
			replayName = fmt.Sprintf("%s-replayed", originalSession.Name)
		} else {
			replayName = fmt.Sprintf("%s-replay-%d", originalSession.Name, time.Now().Unix())
		}

		// Start a new session with the original command and workspace
		newSession, err := sm.StartSession(replayName, originalSession.Command, originalSession.Workspace)
		if err != nil {
			return fmt.Errorf("failed to start replay session: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Successfully started replay session '%s' (PID: %d).\n", newSession.Name, newSession.PID)
		fmt.Fprintf(cmd.OutOrStdout(), "Logs are available at: %s\n", newSession.LogFile)
		return nil
	},
}
