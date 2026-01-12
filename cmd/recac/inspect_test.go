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
	// Setup mock session manager for all sub-tests
	sm := NewMockSessionManager()
	oldFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return sm, nil
	}
	defer func() { sessionManagerFactory = oldFactory }()

	// --- Setup common resources for multiple tests ---
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

	logFile := filepath.Join(tempDir, "completed.log")
	os.WriteFile(logFile, []byte("log line 1\nlog line 2\n"), 0644)

	// --- Test Case 1: Completed session (Specific) ---
	t.Run("InspectCompletedSession", func(t *testing.T) {
		// Reset mock state for this test
		sm.Sessions = make(map[string]*runner.SessionState)

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
	})

	// --- Test Case 2: Running session (Specific) ---
	t.Run("InspectRunningSession", func(t *testing.T) {
		// Reset mock state
		sm.Sessions = make(map[string]*runner.SessionState)
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
	})

	// --- Test Case 3: Errored session (Specific) ---
	t.Run("InspectErroredSession", func(t *testing.T) {
		// Reset mock state
		sm.Sessions = make(map[string]*runner.SessionState)
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
		// Reset mock state
		sm.Sessions = make(map[string]*runner.SessionState)

		// Execute
		_, err := executeCommand(rootCmd, "inspect", "no-such-session")

		// Assert
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), "session not found"), "Expected error to contain 'session not found', but it was: %s", err.Error())
	})

	// --- Test Case 5: Most Recent (No Args) ---
	t.Run("InspectMostRecent", func(t *testing.T) {
		// Reset mock state
		sm.Sessions = make(map[string]*runner.SessionState)

		// Setup: Create multiple sessions with different start times
		oldSession := &runner.SessionState{
			Name:      "old-session",
			Status:    "completed",
			StartTime: time.Now().Add(-2 * time.Hour),
		}
		recentSession := &runner.SessionState{
			Name:      "recent-session",
			Status:    "running",
			StartTime: time.Now().Add(-1 * time.Minute), // Most recent
		}
		middleSession := &runner.SessionState{
			Name:      "middle-session",
			Status:    "completed",
			StartTime: time.Now().Add(-30 * time.Minute),
		}
		sm.Sessions["old-session"] = oldSession
		sm.Sessions["recent-session"] = recentSession
		sm.Sessions["middle-session"] = middleSession

		// Execute command with NO arguments
		output, err := executeCommand(rootCmd, "inspect")

		// Assert
		require.NoError(t, err)
		// It should find and display the most recent session
		require.Contains(t, output, "Session Details for 'recent-session'")
		require.Regexp(t, `Status:\s+running`, output)
		require.NotContains(t, output, "old-session")
	})

	// --- Test Case 6: No sessions found (No Args) ---
	t.Run("InspectNoSessions", func(t *testing.T) {
		// Reset mock state
		sm.Sessions = make(map[string]*runner.SessionState)

		// Execute
		output, err := executeCommand(rootCmd, "inspect")

		// Assert
		require.NoError(t, err)
		require.Contains(t, output, "No sessions found.")
	})
}
