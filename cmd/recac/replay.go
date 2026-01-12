package main

import (
	"fmt"
	"recac/internal/runner"
	"regexp"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(replayCmd)
}

var replayCmd = &cobra.Command{
	Use:   "replay [SESSION_NAME]",
	Short: "Replay a previous session",
	Long:  `Re-runs a previous session using the same configuration.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionToReplay := args[0]

		sm, err := runner.NewSessionManager()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %w", err)
		}

		// 1. Load the original session state
		originalState, err := sm.LoadSession(sessionToReplay)
		if err != nil {
			return fmt.Errorf("failed to load session '%s': %w", sessionToReplay, err)
		}

		// 2. Generate a unique name for the new session
		replayedName, err := generateUniqueReplayName(sm, sessionToReplay)
		if err != nil {
			return fmt.Errorf("failed to generate unique name for replay: %w", err)
		}

		// 3. Modify the original command to use the new name
		replayedCommand := modifyCommandForReplay(originalState.Command, sessionToReplay, replayedName)

		// 4. Start the new session
		replayedSession, err := sm.StartSession(replayedName, replayedCommand, originalState.Workspace)
		if err != nil {
			return fmt.Errorf("failed to start replayed session: %w", err)
		}

		fmt.Printf("Successfully replayed session '%s' as '%s' (PID: %d)\n", sessionToReplay, replayedName, replayedSession.PID)
		fmt.Printf("Log file: %s\n", replayedSession.LogFile)

		return nil
	},
}

// generateUniqueReplayName finds a unique name for the replayed session, e.g., "my-session-replay-1", "my-session-replay-2"
func generateUniqueReplayName(sm *runner.SessionManager, originalName string) (string, error) {
	baseName := originalName
	// If the original name itself is a replay, strip the suffix to avoid names like "session-replay-1-replay-1"
	re := regexp.MustCompile(`-replay-\d+$`)
	if re.MatchString(originalName) {
		baseName = re.Split(originalName, -1)[0]
	}

	for i := 1; i < 100; i++ { // Limit to 99 replays to prevent infinite loops
		replayName := fmt.Sprintf("%s-replay-%d", baseName, i)
		_, err := sm.LoadSession(replayName)
		if err != nil {
			// If loading fails, the session name is available
			return replayName, nil
		}
	}
	return "", fmt.Errorf("could not find an available replay name for '%s'", originalName)
}

// modifyCommandForReplay replaces the old session name with the new one in the command arguments.
func modifyCommandForReplay(originalCommand []string, oldName, newName string) []string {
	replayedCommand := make([]string, len(originalCommand))
	copy(replayedCommand, originalCommand)

	// Find the "--name" flag and replace its value.
	for i := 0; i < len(replayedCommand)-1; i++ {
		if replayedCommand[i] == "--name" && replayedCommand[i+1] == oldName {
			replayedCommand[i+1] = newName
			return replayedCommand
		}
	}

	// If "--name" wasn't found (e.g., interactive session), append it.
	return append(replayedCommand, "--name", newName)
}
