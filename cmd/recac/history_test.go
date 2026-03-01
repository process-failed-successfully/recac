package main

import (
	"fmt"
	"recac/internal/runner"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHistoryCmd_Success(t *testing.T) {
	cmd := rootCmd
	resetFlags(cmd)

	mockSM := NewMockSessionManager()
	mockSM.Sessions["test-session"] = &runner.SessionState{
		Name:      "test-session",
		Status:    "completed",
		Workspace: "/tmp/test",
		StartTime: time.Now().Add(-1 * time.Hour),
		EndTime:   time.Now(),
		Goal:      "Implement feature",
	}

	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return mockSM, nil
	}
	defer func() { sessionManagerFactory = originalFactory }()

	output, err := executeCommand(cmd, "history", "test-session")

	require.NoError(t, err)
	assert.Contains(t, output, "Session Details for 'test-session'")
	assert.Contains(t, output, "Name:")
	assert.Contains(t, output, "Status:")
	assert.Contains(t, output, "Workspace:")
}

func TestHistoryCmd_SessionNotFound(t *testing.T) {
	cmd := rootCmd
	resetFlags(cmd)

	mockSM := NewMockSessionManager()

	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return mockSM, nil
	}
	defer func() { sessionManagerFactory = originalFactory }()

	_, err := executeCommand(cmd, "history", "missing-session")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load session 'missing-session'")
}

func TestHistoryCmd_InitError(t *testing.T) {
	cmd := rootCmd
	resetFlags(cmd)

	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return nil, fmt.Errorf("mock init error")
	}
	defer func() { sessionManagerFactory = originalFactory }()

	_, err := executeCommand(cmd, "history", "test-session")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to initialize session manager: mock init error")
}
