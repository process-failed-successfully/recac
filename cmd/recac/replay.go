package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(replayCmd)
}

var replayCmd = &cobra.Command{
	Use:   "replay [session-name]",
	Short: "Replay a previous session",
	Long: `Replay a previous session by starting a new one with the same command, workspace, and initial git state.
The workspace will be checked out to the starting commit of the original session before execution.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionName := args[0]

		sm, err := sessionManagerFactory()
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
			return errors.New("cannot replay a running session, please stop it first")
		}

		// Restore original git state if possible
		if originalSession.StartCommitSHA != "" {
			gitClient := gitClientFactory()
			fmt.Fprintf(cmd.OutOrStdout(), "Restoring workspace to original commit: %s...\n", originalSession.StartCommitSHA)
			if err := gitClient.Checkout(originalSession.Workspace, originalSession.StartCommitSHA); err != nil {
				// Don't fail the whole operation, just warn
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to checkout original commit: %v. Continuing replay...\n", err)
			}
		}

		// Find the next available replay name
		replayName, err := findNextReplayName(sm, originalSession.Name)
		if err != nil {
			return err
		}

		// Start a new session with the original command and workspace
		newSession, err := sm.StartSession(replayName, originalSession.Goal, originalSession.Command, originalSession.Workspace)
		if err != nil {
			return fmt.Errorf("failed to start replay session: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Successfully started replay session '%s' (PID: %d).\n", newSession.Name, newSession.PID)
		fmt.Fprintf(cmd.OutOrStdout(), "Logs are available at: %s\n", newSession.LogFile)
		return nil
	},
}

// findNextReplayName determines the next available name for a replayed session.
// It looks for existing sessions named `[baseName]-replay-N` and returns the next integer suffix.
func findNextReplayName(sm ISessionManager, baseName string) (string, error) {
	sessions, err := sm.ListSessions()
	if err != nil {
		return "", fmt.Errorf("could not list existing sessions: %w", err)
	}

	prefix := baseName + "-replay-"
	maxReplayNum := 0
	for _, s := range sessions {
		if strings.HasPrefix(s.Name, prefix) {
			var replayNum int
			// Parse the number after the prefix
			if _, err := fmt.Sscanf(s.Name, prefix+"%d", &replayNum); err == nil {
				if replayNum > maxReplayNum {
					maxReplayNum = replayNum
				}
			}
		}
	}

	// To handle the case where we are replaying a non-replay session for the first time
	// we need to check if a replay with suffix '1' already exists
	firstReplayName := fmt.Sprintf("%s-replay-1", baseName)
	foundFirst := false
	for _, s := range sessions {
		if s.Name == firstReplayName {
			foundFirst = true
			break
		}
	}
	if !foundFirst && maxReplayNum == 0 {
		return firstReplayName, nil
	}

	return fmt.Sprintf("%s%d", prefix, maxReplayNum+1), nil
}
