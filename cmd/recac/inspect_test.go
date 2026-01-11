package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"recac/internal/runner"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestInspectCmd(t *testing.T) {
	// Setup mock session manager
	sm := NewMockSessionManager()
	oldFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return sm, nil
	}
	defer func() { sessionManagerFactory = oldFactory }()

	// --- Test Case 1: Completed session ---
	t.Run("InspectCompletedSession", func(t *testing.T) {
		// Setup: Create a temporary directory for the agent state file.
		tempDir := t.TempDir()
		agentStateFile := filepath.Join(tempDir, ".agent_state.json")
		state := &agent.State{
			Model: "test-model",
			TokenUsage: agent.TokenUsage{
				TotalPromptTokens:     100,
				TotalResponseTokens: 200,
				TotalTokens:         300,
			},
		}
		stateBytes, _ := json.Marshal(state)
		os.WriteFile(agentStateFile, stateBytes, 0644)

		// Create a dummy log file
		logFile := filepath.Join(tempDir, "completed.log")
		os.WriteFile(logFile, []byte("log line 1\nlog line 2\n"), 0644)

		completedSession := &runner.SessionState{
			Name:           "completed-session",
			Status:         "completed",
			StartTime:      time.Now().Add(-1 * time.Hour),
			EndTime:        time.Now(),
			Workspace:      "/tmp/workspace1",
			LogFile:        logFile,
			AgentStateFile: agentStateFile,
			StartCommitSHA: "abc1234",
			EndCommitSHA:   "def5678",
		}
		sm.Sessions["completed-session"] = completedSession

		// Execute
		output, err := executeCommand(rootCmd, "inspect", "completed-session")

		// Assert
		require.NoError(t, err)
		require.Contains(t, output, "Session Details for 'completed-session'")
		require.Regexp(t, `Status:\s+completed`, output)
		require.Regexp(t, `Model:\s+test-model`, output)
		require.Regexp(t, `Total Tokens:\s+300`, output)
		require.Contains(t, output, "Git Changes (stat)")
		require.Contains(t, output, "M README.md")
		require.Contains(t, output, "Recent Logs")
	})

	// --- Test Case 2: Running session ---
	t.Run("InspectRunningSession", func(t *testing.T) {
		runningSession := &runner.SessionState{
			Name:      "running-session",
			Status:    "running",
			PID:       12345,
			StartTime: time.Now().Add(-10 * time.Minute),
		}
		sm.Sessions["running-session"] = runningSession

		// Execute
		output, err := executeCommand(rootCmd, "inspect", "running-session")

		// Assert
		require.NoError(t, err)
		require.Contains(t, output, "Session Details for 'running-session'")
		require.Regexp(t, `Status:\s+running`, output)
		require.NotContains(t, output, "End Time:")
		require.NotContains(t, output, "Git Changes (stat)") // No end commit sha
	})

	// --- Test Case 3: Errored session ---
	t.Run("InspectErroredSession", func(t *testing.T) {
		erroredSession := &runner.SessionState{
			Name:      "errored-session",
			Status:    "error",
			StartTime: time.Now().Add(-5 * time.Minute),
			EndTime:   time.Now(),
			Error:     "something went terribly wrong",
		}
		sm.Sessions["errored-session"] = erroredSession

		// Execute
		output, err := executeCommand(rootCmd, "inspect", "errored-session")

		// Assert
		require.NoError(t, err)
		require.Contains(t, output, "Session Details for 'errored-session'")
		require.Regexp(t, `Status:\s+error`, output)
		require.Regexp(t, `Error:\s+something went terribly wrong`, output)
	})

	// --- Test Case 4: Non-existent session ---
	t.Run("InspectNonExistentSession", func(t *testing.T) {
		// Execute
		_, err := executeCommand(rootCmd, "inspect", "no-such-session")

		// Assert
		require.Error(t, err)
		// This is the error from the mock manager, confirming it was called.
		require.True(t, strings.Contains(err.Error(), "session not found"), "Expected error to contain 'session not found', but it was: %s", err.Error())
	})

	// --- Test Case 5: No args ---
	t.Run("InspectNoArgs", func(t *testing.T) {
		// Execute
		_, err := executeCommand(rootCmd, "inspect")
		// Assert
		require.Error(t, err)
		require.Contains(t, err.Error(), "accepts 1 arg(s), received 0")
	})
}
