package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"recac/internal/runner"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupListTest creates a temporary directory and injects a mock session manager for the list command.
func setupListTest(t *testing.T) (string, func()) {
	tempDir, err := os.MkdirTemp("", "recac-list-test-*")
	require.NoError(t, err)

	sm, err := runner.NewSessionManagerWithDir(tempDir)
	require.NoError(t, err)

	// Override the factory function to inject the mock session manager
	originalNewSessionManager := newSessionManager
	newSessionManager = func() (*runner.SessionManager, error) {
		return sm, nil
	}

	cleanup := func() {
		os.RemoveAll(tempDir)
		newSessionManager = originalNewSessionManager // Restore original factory
	}

	return tempDir, cleanup
}

func TestListCommand(t *testing.T) {
	tempDir, cleanup := setupListTest(t)
	defer cleanup()

	// Use a valid, running PID for running sessions
	runningPID := os.Getpid()

	// --- Create Mock Sessions ---
	session1 := &runner.SessionState{Name: "session-b", PID: runningPID, Status: "running", StartTime: time.Now().Add(-1 * time.Hour)}
	session2 := &runner.SessionState{Name: "session-a", PID: 0, Status: "completed", StartTime: time.Now().Add(-2 * time.Hour)} // PID 0 for completed
	session3 := &runner.SessionState{Name: "session-c", PID: runningPID, Status: "running", StartTime: time.Now()}

	createMockSessionFile(t, tempDir, session1)
	createMockSessionFile(t, tempDir, session2)
	createMockSessionFile(t, tempDir, session3)

	t.Run("Default output sorted by start time", func(t *testing.T) {
		output, _ := executeCommand(rootCmd, "list")
		assert.Contains(t, output, "session-a")
		assert.Contains(t, output, "session-b")
		// session2 started first, so it should appear first in the default sort
		assert.True(t, strings.Index(output, "session-a") < strings.Index(output, "session-b"), "Expected session-a to appear before session-b")
	})

	t.Run("Filter by status", func(t *testing.T) {
		output, _ := executeCommand(rootCmd, "list", "--status", "running")
		assert.Contains(t, output, "session-b")
		assert.Contains(t, output, "session-c")
		assert.NotContains(t, output, "session-a")
	})

	t.Run("Sort by name", func(t *testing.T) {
		output, _ := executeCommand(rootCmd, "list", "--sort-by", "name")
		assert.True(t, strings.Index(output, "session-a") < strings.Index(output, "session-b"), "Expected session-a to appear before session-b")
	})

	t.Run("JSON output", func(t *testing.T) {
		output, _ := executeCommand(rootCmd, "list", "--json")
		var sessions []*runner.SessionState
		err := json.Unmarshal([]byte(output), &sessions)
		require.NoError(t, err, "Output should be valid JSON")
		assert.Len(t, sessions, 3)
	})

	t.Run("JSON output with filter", func(t *testing.T) {
		output, _ := executeCommand(rootCmd, "list", "--status", "completed", "--json")
		var sessions []*runner.SessionState
		err := json.Unmarshal([]byte(output), &sessions)
		require.NoError(t, err, "Output should be valid JSON")
		require.Len(t, sessions, 1)
		assert.Equal(t, "session-a", sessions[0].Name)
	})
}

func TestListCommand_NoSessions(t *testing.T) {
	_, cleanup := setupListTest(t)
	defer cleanup()

	t.Run("No sessions standard output", func(t *testing.T) {
		output, _ := executeCommand(rootCmd, "list")
		assert.Contains(t, output, "No sessions found.")
	})

	t.Run("No sessions JSON output", func(t *testing.T) {
		output, _ := executeCommand(rootCmd, "list", "--json")
		assert.JSONEq(t, "[]", strings.TrimSpace(output))
	})
}

// createMockSessionFile is a helper to create session files for testing.
func createMockSessionFile(t *testing.T, dir string, session *runner.SessionState) {
	t.Helper()
	data, err := json.Marshal(session)
	require.NoError(t, err)
	sessionFile := filepath.Join(dir, session.Name+".json")
	err = os.WriteFile(sessionFile, data, 0644)
	require.NoError(t, err)
}
