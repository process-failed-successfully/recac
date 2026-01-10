package main

import (
	"fmt"
	"recac/internal/runner"
	"time"

	"github.com/spf13/cobra"
)

// newSessionManager is a function variable that can be overridden in tests
var newSessionManager = runner.NewSessionManager

func init() {
	rootCmd.AddCommand(replayCmd)
}

var replayCmd = &cobra.Command{
	Use:   "replay [session-name]",
	Short: "Replay a previous session",
	Long:  `Replay a previous session by starting a new one with the same command and workspace.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		sessionName := args[0]

		sm, err := newSessionManager()
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error: failed to create session manager: %v\n", err)
			exit(1)
			return
		}

		// Load the original session
		originalSession, err := sm.LoadSession(sessionName)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error: failed to load session '%s': %v\n", sessionName, err)
			exit(1)
			return
		}

		// Prevent replaying a running session to avoid unexpected behavior
		if originalSession.Status == "running" && sm.IsProcessRunning(originalSession.PID) {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error: cannot replay a running session. Please stop it first.\n")
			exit(1)
			return
		}

		// Create a new name for the replayed session
		replayName := fmt.Sprintf("%s-replay-%d", originalSession.Name, time.Now().Unix())

		// Start a new session with the original command and workspace
		newSession, err := sm.StartSession(replayName, originalSession.Command, originalSession.Workspace)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error: failed to start replay session: %v\n", err)
			exit(1)
			return
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Successfully started replay session '%s' (PID: %d).\n", newSession.Name, newSession.PID)
		fmt.Fprintf(cmd.OutOrStdout(), "Logs are available at: %s\n", newSession.LogFile)
	},
}
