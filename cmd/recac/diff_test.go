package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"recac/internal/agent"
	"recac/internal/runner"
	"github.com/stretchr/testify/require"
)

func setupDiffTest(t *testing.T, sm *MockSessionManager) {
	t.Helper()

	// Create a temporary directory for logs and state files
	tempDir := t.TempDir()

	// --- Create Session A ---
	sessionA := &runner.SessionState{
		Name:      "session-a",
		PID:       12345,
		StartTime: time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2023, 1, 1, 10, 5, 0, 0, time.UTC),
		Status:    "completed",
		LogFile:   filepath.Join(tempDir, "session-a.log"),
		AgentStateFile: filepath.Join(tempDir, "session-a.agent_state.json"),
	}
	err := os.WriteFile(sessionA.LogFile, []byte("line 1\ncommon line\nline 3a\n"), 0600)
	require.NoError(t, err)
	agentStateA := &agent.State{
		Model: "gemini-pro",
		TokenUsage: agent.TokenUsage{TotalPromptTokens: 100, TotalResponseTokens: 200},
	}
	stateDataA, _ := json.Marshal(agentStateA)
	err = os.WriteFile(sessionA.AgentStateFile, stateDataA, 0600)
	require.NoError(t, err)
	sm.Sessions[sessionA.Name] = sessionA


	// --- Create Session B ---
	sessionB := &runner.SessionState{
		Name:      "session-b",
		PID:       54321,
		StartTime: time.Date(2023, 1, 1, 11, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2023, 1, 1, 11, 18, 0, 0, time.UTC),
		Status:    "error",
		Error:     "Something went wrong",
		LogFile:   filepath.Join(tempDir, "session-b.log"),
		AgentStateFile: filepath.Join(tempDir, "session-b.agent_state.json"),
	}
	err = os.WriteFile(sessionB.LogFile, []byte("line 1\ncommon line\nline 3b\n"), 0600)
	require.NoError(t, err)
	agentStateB := &agent.State{
		Model: "gpt-4",
		TokenUsage: agent.TokenUsage{TotalPromptTokens: 300, TotalResponseTokens: 400},
	}
	stateDataB, _ := json.Marshal(agentStateB)
	err = os.WriteFile(sessionB.AgentStateFile, stateDataB, 0600)
	require.NoError(t, err)
	sm.Sessions[sessionB.Name] = sessionB

	// --- Create Identical Sessions (for no diff test) ---
	sessionC := &runner.SessionState{
		Name: "session-c",
		LogFile:   filepath.Join(tempDir, "session-c.log"),
		AgentStateFile: filepath.Join(tempDir, "session-c.agent_state.json"),
	}
	err = os.WriteFile(sessionC.LogFile, []byte("identical line\n"), 0600)
	require.NoError(t, err)
	err = os.WriteFile(sessionC.AgentStateFile, []byte("{}"), 0600) // Empty state
	require.NoError(t, err)
	sm.Sessions[sessionC.Name] = sessionC

	sessionD := &runner.SessionState{
		Name: "session-d",
		LogFile:   filepath.Join(tempDir, "session-d.log"),
		AgentStateFile: filepath.Join(tempDir, "session-d.agent_state.json"),
	}
	err = os.WriteFile(sessionD.LogFile, []byte("identical line\n"), 0600)
	require.NoError(t, err)
	err = os.WriteFile(sessionD.AgentStateFile, []byte("{}"), 0600) // Empty state
	require.NoError(t, err)
	sm.Sessions[sessionD.Name] = sessionD
}

func TestDiffCmd(t *testing.T) {
	// Setup mock session manager
	mockSM := NewMockSessionManager()
	setupDiffTest(t, mockSM)

	// Override the factory to return our mock manager
	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return mockSM, nil
	}
	defer func() { sessionManagerFactory = originalFactory }()

	t.Run("displays metadata and log diff", func(t *testing.T) {
		// Execute the command
		output, err := executeCommand(rootCmd, "diff", "session-a", "session-b")
		require.NoError(t, err)

		// Assert Metadata
		require.Contains(t, output, "📊 Metadata Comparison")
		require.Contains(t, output, "session-a")
		require.Contains(t, output, "completed")
		require.Contains(t, output, "5m0s")
		require.Contains(t, output, "gemini-pro")
		require.Contains(t, output, "300")
		require.Contains(t, output, "session-b")
		require.Contains(t, output, "error")
		require.Contains(t, output, "18m0s")
		require.Contains(t, output, "gpt-4")
		require.Contains(t, output, "700")
		require.Contains(t, output, "Something went wrong")


		// Assert Log Diff
		require.Contains(t, output, "📜 Log Diff")
		require.Contains(t, output, "-line 3a")
		require.Contains(t, output, "+line 3b")
	})

	t.Run("handles no differences in logs", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "diff", "session-c", "session-d")
		require.NoError(t, err)
		require.Contains(t, output, "No differences in logs.")
	})

	t.Run("handles missing sessions", func(t *testing.T) {
		_, err := executeCommand(rootCmd, "diff", "session-a", "non-existent")
		require.Error(t, err)
		require.Contains(t, err.Error(), "session not found")

		_, err = executeCommand(rootCmd, "diff", "non-existent", "session-b")
		require.Error(t, err)
		require.Contains(t, err.Error(), "session not found")
	})
}
