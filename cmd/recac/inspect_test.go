package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
	"recac/internal/agent"
	"recac/internal/runner"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInspectCmd(t *testing.T) {
	tempDir := t.TempDir()
	sessionsDir := filepath.Join(tempDir, ".recac", "sessions")
	err := os.MkdirAll(sessionsDir, 0755)
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

	// Create a mock session manager
	mockSM := &MockSessionManager{
		Sessions: make(map[string]*runner.SessionState),
	}

	// Create a fake session
	sessionName := "test-session"
	fakeSession := &runner.SessionState{
		Name:      sessionName,
		Status:    "completed",
		PID:       12345,
		Type:      "test",
		StartTime: time.Now().Add(-1 * time.Hour),
		EndTime:   time.Now(),
		Workspace: "/tmp/workspace",
		LogFile:   "/tmp/test.log",
		Error:     "",
		AgentStateFile: agentStateFile,
	}
	err = mockSM.SaveSession(fakeSession)
	require.NoError(t, err)

	// Replace the sessionManagerFactory with a mock factory
	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return mockSM, nil
	}
	defer func() { sessionManagerFactory = originalFactory }()

	// Execute the inspect command
	output, err := executeCommand(rootCmd, "inspect", sessionName)
	require.NoError(t, err)

	// Check the output
	assert.Contains(t, output, fmt.Sprintf("Session Details for '%s'", sessionName))
	assert.Regexp(t, `Name:\s+test-session`, output)
	assert.Regexp(t, `Status:\s+completed`, output)
	assert.Regexp(t, `PID:\s+12345`, output)
	assert.Regexp(t, `Type:\s+test`, output)
	assert.Contains(t, output, "Agent & Token Usage")
	assert.Regexp(t, `Model:\s+test-model`, output)
	assert.Regexp(t, `Total Tokens:\s+300`, output)
}
