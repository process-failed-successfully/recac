package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"recac/internal/agent"
	"recac/internal/runner"
	"github.com/stretchr/testify/require"
)

// TestHistoryCommand tests the enhanced history command.
func TestHistoryCommand(t *testing.T) {
	// 1. Setup Mock Session
	tempDir := t.TempDir()
	sessionsDir := filepath.Join(tempDir, ".recac", "sessions")
	err := os.MkdirAll(sessionsDir, 0755)
	require.NoError(t, err)

	// Create mock session manager
	sm, err := runner.NewSessionManagerWithDir(sessionsDir)
	require.NoError(t, err)

	// Create mock log file
	logContent := "line 1\nline 2\nline 3\nline 4\nline 5\nline 6\nline 7\nline 8\nline 9\nline 10\nline 11"
	logFile := filepath.Join(sessionsDir, "test-session.log")
	err = os.WriteFile(logFile, []byte(logContent), 0644)
	require.NoError(t, err)

	// Create mock agent state file
	agentState := agent.State{
		Model: "test-model",
		TokenUsage: agent.TokenUsage{
			TotalPromptTokens:     100,
			TotalResponseTokens: 200,
			TotalTokens:          300,
		},
	}
	agentStateContent, err := json.Marshal(agentState)
	require.NoError(t, err)
	agentStateFile := filepath.Join(sessionsDir, ".agent_state.json")
	err = os.WriteFile(agentStateFile, agentStateContent, 0644)
	require.NoError(t, err)

	// Create and save mock session state
	session := &runner.SessionState{
		Name:           "test-session",
		Status:         "completed",
		PID:            12345,
		Type:           "detached",
		StartTime:      time.Now().Add(-1 * time.Hour),
		EndTime:        time.Now(),
		Workspace:      "/tmp/workspace",
		LogFile:        logFile,
		AgentStateFile: agentStateFile,
		Error:          "test error",
	}
	sessionPath := sm.GetSessionPath("test-session")
	sessionContent, err := json.Marshal(session)
	require.NoError(t, err)
	err = os.WriteFile(sessionPath, sessionContent, 0644)
	require.NoError(t, err)

	// 2. Setup Cobra Command
	rootCmd, _, _ := newRootCmd()
	initHistoryCmd(rootCmd)

	// Mock the sessionManagerFactory
	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return sm, nil
	}
	defer func() { sessionManagerFactory = originalFactory }()

	// 3. Run and Assert: Standard History (last 10 lines)
	t.Run("Standard History", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "history", "test-session")
		require.NoError(t, err)
		// Check for session details
		require.Contains(t, output, "Session Details for 'test-session'")
		require.Regexp(t, `Status:\s+completed`, output)
		require.Regexp(t, `Error:\s+test error`, output)

		// Check for agent state
		require.Contains(t, output, "Agent & Token Usage")
		require.Regexp(t, `Model:\s+test-model`, output)
		require.Regexp(t, `Total Tokens:\s+300`, output)

		// Check for recent logs (last 10 lines)
		require.Contains(t, output, "Recent Logs (last 10 lines)")
		require.NotContains(t, output, "line 1\n") // Should not be present
		require.Contains(t, output, "line 11")
		require.Equal(t, 10, strings.Count(output[strings.Index(output, "Recent Logs"):], "\n")-2) // 10 log lines
	})

	// 4. Run and Assert: Full Logs
	t.Run("Full Logs History", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "history", "test-session", "--full-logs")
		require.NoError(t, err)
		// Check for full logs header
		require.Contains(t, output, "Full Logs")
		// Check that all lines are present
		require.Contains(t, output, "line 1\n")
		require.Contains(t, output, "line 11")
		// 11 log lines + header/separator
		require.Equal(t, 11, strings.Count(output[strings.Index(output, "Full Logs"):], "\n")-2)
	})
}
