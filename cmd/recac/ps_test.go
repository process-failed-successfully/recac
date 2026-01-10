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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPsAndListCommands(t *testing.T) {
	tempDir := t.TempDir()
	sessionDir := filepath.Join(tempDir, "sessions")
	os.MkdirAll(sessionDir, 0755)

	oldSessionManager := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return runner.NewSessionManagerWithDir(sessionDir)
	}
	defer func() { sessionManagerFactory = oldSessionManager }()

	testCases := []struct {
		name    string
		command string
	}{
		{
			name:    "ps command with no sessions",
			command: "ps",
		},
		{
			name:    "list alias with no sessions",
			command: "list",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output, err := executeCommand(rootCmd, tc.command)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			if !strings.Contains(output, "No sessions found.") {
				t.Errorf("expected output to contain 'No sessions found.', got '%s'", output)
			}
		})
	}
}

func TestPsCommandWithCosts(t *testing.T) {
	tempDir := t.TempDir()
	sessionsDir := filepath.Join(tempDir, "sessions")
	require.NoError(t, os.Mkdir(sessionsDir, 0755))

	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return runner.NewSessionManagerWithDir(sessionsDir)
	}
	defer func() { sessionManagerFactory = originalFactory }()

	sm, err := sessionManagerFactory()
	require.NoError(t, err)

	sessionWithCost := &runner.SessionState{
		Name:           "session-with-cost",
		Status:         "completed",
		StartTime:      time.Now().Add(-1 * time.Hour),
		EndTime:        time.Now(),
		AgentStateFile: filepath.Join(sessionsDir, "session-with-cost-agent-state.json"),
	}
	sessionWithoutCost := &runner.SessionState{
		Name:           "session-without-cost",
		Status:         "running",
		StartTime:      time.Now().Add(-10 * time.Minute),
		AgentStateFile: filepath.Join(sessionsDir, "non-existent-agent-state.json"),
	}

	agentState := &agent.State{
		Model: "gemini-pro",
		TokenUsage: agent.TokenUsage{
			TotalPromptTokens:   1000,
			TotalResponseTokens: 2000,
			TotalTokens:         3000,
		},
	}
	stateData, err := json.Marshal(agentState)
	require.NoError(t, err)
	err = os.WriteFile(sessionWithCost.AgentStateFile, stateData, 0644)
	require.NoError(t, err)

	err = sm.SaveSession(sessionWithCost)
	require.NoError(t, err)
	err = sm.SaveSession(sessionWithoutCost)
	require.NoError(t, err)

	output, err := executeCommand(rootCmd, "ps", "--costs")
	require.NoError(t, err)

	assert.Contains(t, output, "NAME")
	assert.Contains(t, output, "STATUS")
	assert.Contains(t, output, "COST")
	assert.Contains(t, output, "TOTAL_TOKENS")

	assert.Regexp(t, `session-with-cost\s+completed`, output)
	assert.Contains(t, output, "1000")
	assert.Contains(t, output, "2000")
	assert.Contains(t, output, "3000")

	assert.Regexp(t, `session-without-cost\s+completed`, output)
	assert.Contains(t, output, "N/A")
}
