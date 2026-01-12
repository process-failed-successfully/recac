package main

import (
	"recac/internal/runner"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFindCmd(t *testing.T) {
	// Setup mock session manager
	mockSM := NewMockSessionManager()
	mockSM.Sessions["test-session-1"] = &runner.SessionState{Name: "test-session-1", Status: "completed", StartTime: time.Now()}
	mockSM.Sessions["test-session-2"] = &runner.SessionState{Name: "test-session-2", Status: "running", StartTime: time.Now()}
	mockSM.Sessions["another-session-3"] = &runner.SessionState{Name: "another-session-3", Status: "error", StartTime: time.Now()}
	mockSM.Sessions["test-session-4"] = &runner.SessionState{Name: "test-session-4", Status: "COMPLETED", StartTime: time.Now()}

	// Replace the factory with our mock
	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return mockSM, nil
	}
	defer func() { sessionManagerFactory = originalFactory }()

	t.Run("find by name", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "find", "--name", "test-session")
		assert.NoError(t, err)
		assert.Contains(t, output, "test-session-1")
		assert.Contains(t, output, "test-session-2")
		assert.Contains(t, output, "test-session-4")
		assert.NotContains(t, output, "another-session-3")
	})

	t.Run("find by status", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "find", "--status", "completed")
		assert.NoError(t, err)
		assert.Contains(t, output, "test-session-1")
		assert.Contains(t, output, "test-session-4") // Should match case-insensitively
		assert.NotContains(t, output, "test-session-2")
		assert.NotContains(t, output, "another-session-3")
	})

	t.Run("find by name and status", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "find", "--name", "test-session", "--status", "running")
		assert.NoError(t, err)
		assert.Contains(t, output, "test-session-2")
		assert.NotContains(t, output, "test-session-1")
		assert.NotContains(t, output, "test-session-4")
		assert.NotContains(t, output, "another-session-3")
	})

	t.Run("find with no filters", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "find")
		assert.NoError(t, err)
		assert.Contains(t, output, "test-session-1")
		assert.Contains(t, output, "test-session-2")
		assert.Contains(t, output, "another-session-3")
		assert.Contains(t, output, "test-session-4")
	})

	t.Run("find with no matches", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "find", "--name", "non-existent")
		assert.NoError(t, err)
		assert.Contains(t, output, "No matching sessions found.")
	})

	t.Run("find by status case insensitive", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "find", "--status", "Completed")
		assert.NoError(t, err)
		// Both should appear because EqualFold is used.
		assert.Contains(t, output, "test-session-1")
		assert.Contains(t, output, "test-session-4")
		assert.Equal(t, 3, strings.Count(output, "\n"), "Should find 2 sessions plus header")
	})
}
