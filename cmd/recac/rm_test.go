package main

import (
	"testing"

	"recac/internal/runner"

	"github.com/stretchr/testify/require"
)

func TestRmCmd(t *testing.T) {
	// Use the shared newRootCmd to ensure flags are reset
	rootCmd, _, _ := newRootCmd()

	// 1. Setup a mock session manager
	mockSM := NewMockSessionManager()
	mockSM.Sessions["session1"] = &runner.SessionState{Name: "session1", Status: "completed", PID: 123}
	mockSM.Sessions["session2"] = &runner.SessionState{Name: "session2", Status: "running", PID: 456}
	mockSM.Sessions["session3"] = &runner.SessionState{Name: "session3", Status: "completed", PID: 789}

	// Override the factory to return our mock
	oldFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return mockSM, nil
	}
	defer func() { sessionManagerFactory = oldFactory }()

	t.Run("remove single session", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "rm", "session1")
		require.NoError(t, err)
		require.Contains(t, output, "Removed session 'session1'")
		_, exists := mockSM.Sessions["session1"]
		require.False(t, exists, "session1 should be removed")
	})

	t.Run("remove multiple sessions", func(t *testing.T) {
		// Re-add session1 for this test case
		mockSM.Sessions["session1"] = &runner.SessionState{Name: "session1", Status: "completed"}
		output, err := executeCommand(rootCmd, "rm", "session1", "session3")
		require.NoError(t, err)
		require.Contains(t, output, "Removed session 'session1'")
		require.Contains(t, output, "Removed session 'session3'")
		_, exists := mockSM.Sessions["session1"]
		require.False(t, exists, "session1 should be removed")
		_, exists = mockSM.Sessions["session3"]
		require.False(t, exists, "session3 should be removed")
	})

	t.Run("attempt to remove running session without force", func(t *testing.T) {
		// session2 is running
		output, err := executeCommand(rootCmd, "rm", "session2")
		require.NoError(t, err)
		require.Contains(t, output, "Skipping running session 'session2'. Use --force to remove.")
		_, exists := mockSM.Sessions["session2"]
		require.True(t, exists, "session2 should not be removed")
	})

	t.Run("remove running session with force", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "rm", "--force", "session2")
		require.NoError(t, err)
		require.Contains(t, output, "Removed session 'session2'")
		_, exists := mockSM.Sessions["session2"]
		require.False(t, exists, "session2 should be removed")
	})

	t.Run("remove non-existent session", func(t *testing.T) {
		_, err := executeCommand(rootCmd, "rm", "nonexistent")
		require.Error(t, err)
		require.Contains(t, err.Error(), "Failed to remove session 'nonexistent'")
	})

	t.Run("no arguments provided", func(t *testing.T) {
		_, err := executeCommand(rootCmd, "rm")
		require.Error(t, err) // Cobra should return an error
	})
}
