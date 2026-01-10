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

func TestTopCommand(t *testing.T) {
	// Setup: Create a temporary directory for sessions
	dir := t.TempDir()
	t.Setenv("HOME", dir) // Isolate session manager

	// Create mock session files
	createMockSessionForTop(t, dir, "session-cheap", "COMPLETED", 100, 200, "gemini-pro")     // low cost
	createMockSessionForTop(t, dir, "session-expensive", "COMPLETED", 50000, 100000, "gpt-4o") // high cost
	createMockSessionForTop(t, dir, "session-medium", "RUNNING", 1000, 2000, "gpt-3.5-turbo") // medium cost
	createMockSessionForTop(t, dir, "session-no-state", "ERROR", 0, 0, "")                   // no agent state

	// Setup Cobra command
	rootCmd, _, _ := newRootCmd()
	rootCmd.SetArgs([]string{"top"})

	// Execute and capture output
	output, err := executeCommand(rootCmd, "")
	require.NoError(t, err)

	// Assertions
	require.Contains(t, output, "session-expensive", "The most expensive session should be listed first")
	require.Contains(t, output, "session-medium", "The medium cost session should be in the output")
	require.Contains(t, output, "session-cheap", "The cheapest session should be in the output")
	require.NotContains(t, output, "session-no-state", "Session without agent state should not be listed")

	// Test the -n flag
	rootCmd.SetArgs([]string{"top", "-n", "2"})
	output, err = executeCommand(rootCmd, "")
	require.NoError(t, err)
	require.Contains(t, output, "session-expensive")
	require.Contains(t, output, "session-medium")
	require.NotContains(t, output, "session-cheap", "The cheapest session should be excluded when n=2")
}

// Helper function to create mock session data
func createMockSessionForTop(t *testing.T, baseDir, name, status string, promptTokens, responseTokens int, model string) {
	sessionDir := filepath.Join(baseDir, ".recac", "sessions", name)
	require.NoError(t, os.MkdirAll(sessionDir, 0755))

	// Session State
	state := &runner.SessionState{
		Name:           name,
		Status:         status,
		StartTime:      time.Now().Add(-1 * time.Hour),
		EndTime:        time.Now(),
		AgentStateFile: filepath.Join(sessionDir, "agent_state.json"),
	}
	stateBytes, _ := json.Marshal(state)
	require.NoError(t, os.WriteFile(filepath.Join(sessionDir, "state.json"), stateBytes, 0644))

	// Agent State (if a model is provided)
	if model != "" {
		agentState := &agent.State{
			Model: model,
			TokenUsage: agent.TokenUsage{
				TotalPromptTokens:   promptTokens,
				TotalResponseTokens: responseTokens,
				TotalTokens:         promptTokens + responseTokens,
			},
		}
		agentBytes, _ := json.Marshal(agentState)
		require.NoError(t, os.WriteFile(state.AgentStateFile, agentBytes, 0644))
	}
}
