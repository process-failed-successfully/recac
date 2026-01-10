package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"recac/internal/runner"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPsCmd_InspectOutput(t *testing.T) {
	// 1. Setup a temporary session directory
	sessionDir := t.TempDir()
	t.Setenv("HOME", t.TempDir()) // Isolate home directory
	sessionsPath := filepath.Join(sessionDir, ".recac", "sessions")
	err := os.MkdirAll(sessionsPath, 0755)
	require.NoError(t, err)

	// Override the factory to use a mock session manager with our temp dir
	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return runner.NewSessionManagerWithDir(sessionsPath)
	}
	defer func() { sessionManagerFactory = originalFactory }()

	// 2. Create mock session state and agent state files
	sessionName := "test-inspect-session"
	startTime := time.Now().Add(-10 * time.Minute)
	agentState := &agent.State{
		Model: "test-model",
		TokenUsage: agent.TokenUsage{
			TotalPromptTokens:     100,
			TotalResponseTokens:   200,
			TotalTokens:           300,
		},
	}
	sessionState := &runner.SessionState{
		Name:           sessionName,
		Status:         "COMPLETED",
		PID:            12345,
		Type:           "test",
		StartTime:      startTime,
		EndTime:        startTime.Add(5 * time.Minute),
		Workspace:      "/tmp/workspace",
		LogFile:        filepath.Join(sessionsPath, sessionName+".log"),
		AgentStateFile: filepath.Join(sessionsPath, sessionName+"_agent_state.json"),
	}

	// Write agent state file
	agentStateBytes, err := json.Marshal(agentState)
	require.NoError(t, err)
	err = os.WriteFile(sessionState.AgentStateFile, agentStateBytes, 0644)
	require.NoError(t, err)

	// Write session state file
	sessionStateBytes, err := json.Marshal(sessionState)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(sessionsPath, sessionName+".json"), sessionStateBytes, 0644)
	require.NoError(t, err)

	// Write a dummy log file
	err = os.WriteFile(sessionState.LogFile, []byte("line 1\nline 2"), 0644)
	require.NoError(t, err)

	// 3. Execute the 'ps <session-name>' command
	output, err := executeCommand(rootCmd, "ps", sessionName)
	require.NoError(t, err)

	// 4. Assert the output contains the expected details
	require.Contains(t, output, "Session Details for 'test-inspect-session'")
	require.Contains(t, output, "Status:			COMPLETED")
	require.Contains(t, output, "Model:			test-model")
	require.Contains(t, output, "Total Tokens:		300")
	require.Contains(t, output, "Estimated Cost:") // Check for the label
	require.Contains(t, output, "Recent Logs (last 10 lines)")
	require.Contains(t, output, "line 1")
	require.Contains(t, output, "line 2")

	// Ensure it does *not* contain the list view header
	require.NotContains(t, output, "NAME\tSTATUS\tSTARTED\tDURATION")
}
