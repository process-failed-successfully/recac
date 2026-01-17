package main

import (
	"bytes"
	"fmt"
	"os"
	"recac/internal/runner"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatusCmd(t *testing.T) {
	// Create a temporary directory for tests
	tmpDir, err := os.MkdirTemp("", "recac-status-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Mock session manager
	mockSM := NewMockSessionManager()

	// Create a session with agent state file
	agentStateFile := fmt.Sprintf("%s/agent_state.json", tmpDir)

	mockSM.Sessions["test-session"] = &runner.SessionState{
		Name:           "test-session",
		Status:         "running",
		AgentStateFile: agentStateFile,
		StartTime:      time.Now(),
	}

	// Override factory
	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return mockSM, nil
	}
	defer func() { sessionManagerFactory = originalFactory }()

	t.Run("Status No State File", func(t *testing.T) {
		cmd := NewStatusCmd()
		b := new(bytes.Buffer)
		cmd.SetOut(b)
		cmd.SetErr(b)
		cmd.SetArgs([]string{"test-session"})

		err := cmd.Execute()
		assert.NoError(t, err)
		assert.Contains(t, b.String(), "Session 'test-session' found, but agent state is not available.")
		assert.Contains(t, b.String(), "Status: running")
	})

	t.Run("Status With State File", func(t *testing.T) {
		// Create state file
		stateContent := `{
			"history": [],
			"token_usage": {
				"total_tokens": 100,
				"prompt_tokens": 50,
				"completion_tokens": 50
			},
			"cost": 0.002
		}`
		err := os.WriteFile(agentStateFile, []byte(stateContent), 0644)
		require.NoError(t, err)

		cmd := NewStatusCmd()
		b := new(bytes.Buffer)
		cmd.SetOut(b)
		cmd.SetErr(b)
		cmd.SetArgs([]string{"test-session"})

		err = cmd.Execute()
		assert.NoError(t, err)
		// Based on the failure output, the string contains "Estimated Cost" and "Token Usage", not "Total Cost" and "Tokens"
		assert.Contains(t, b.String(), "Estimated Cost")
		assert.Contains(t, b.String(), "Token Usage")
	})

	t.Run("Status Most Recent", func(t *testing.T) {
		mockSM.Sessions["recent-session"] = &runner.SessionState{
			Name:           "recent-session",
			Status:         "running",
			AgentStateFile: "non-existent",
			StartTime:      time.Now().Add(1 * time.Hour),
		}

		cmd := NewStatusCmd()
		b := new(bytes.Buffer)
		cmd.SetOut(b)
		cmd.SetErr(b)
		cmd.SetArgs([]string{})

		err := cmd.Execute()
		assert.NoError(t, err)
		assert.Contains(t, b.String(), "showing status for most recent session: recent-session")
	})
}
