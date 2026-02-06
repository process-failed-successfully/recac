package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"recac/internal/runner"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetUnifiedSessions(t *testing.T) {
	// Create a temporary directory for sessions
	tmpDir := t.TempDir()
    workspaceDir := t.TempDir()

	// Initialize SessionManager with the temp directory
	sm, err := runner.NewSessionManagerWithDir(tmpDir)
	require.NoError(t, err)

	// Create a dummy session state file
	session := &runner.SessionState{
		Name:      "test-session",
		Status:    "running",
		StartTime: time.Now(),
		Type:      "local",
		PID:       os.Getpid(), // Use current process PID
		Goal:      "Fix a bug",
        AgentStateFile: filepath.Join(workspaceDir, "agent_state.json"),
	}
	err = sm.SaveSession(session)
	require.NoError(t, err)

    // Create a dummy agent state file in the separate workspace
    agentState := &agent.State{
        Model: "gpt-4",
        TokenUsage: agent.TokenUsage{
            TotalTokens: 100,
        },
        LastActivity: time.Now(),
        History: []agent.Message{
            {Role: "user", Content: "Fix a bug in the code."},
        },
    }
    agentData, err := json.Marshal(agentState)
    require.NoError(t, err)
    err = os.WriteFile(session.AgentStateFile, agentData, 0644)
    require.NoError(t, err)

	// Call getUnifiedSessions
	sessions, err := getUnifiedSessions(sm)
	require.NoError(t, err)

	// Verify results
	assert.Len(t, sessions, 1)
	s := sessions[0]
	assert.Equal(t, "test-session", s.Name)
	assert.Equal(t, "running", s.Status)
	assert.Equal(t, "local", s.Location)
    assert.Equal(t, "Fix a bug", s.Goal)
    assert.True(t, s.HasCost)
}

func TestGetUnifiedSessions_Docker(t *testing.T) {
	// Create a temporary directory for sessions
	tmpDir := t.TempDir()

	// Initialize SessionManager with the temp directory
	sm, err := runner.NewSessionManagerWithDir(tmpDir)
	require.NoError(t, err)

	// Create a dummy docker session state file
	session := &runner.SessionState{
		Name:        "docker-session",
		Status:      "running",
		StartTime:   time.Now(),
		Type:        "orchestrated-docker",
		ContainerID: "container-123",
        Goal:        "Implement feature X",
	}
	err = sm.SaveSession(session)
	require.NoError(t, err)

	// Call getUnifiedSessions
	sessions, err := getUnifiedSessions(sm)
	require.NoError(t, err)

	// Verify results
	assert.Len(t, sessions, 1)
	s := sessions[0]
	assert.Equal(t, "docker-session", s.Name)
	assert.Equal(t, "running", s.Status)
	assert.Equal(t, "docker", s.Location) // Should be mapped to docker
    assert.Equal(t, "Implement feature X", s.Goal)
    assert.Equal(t, "N/A", s.CPU) // CPU should be N/A for docker
}
