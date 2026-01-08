package main

import (
	"os"
	"path/filepath"
	"recac/internal/runner"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// createMockSession creates a mock session file and a corresponding log file.
func createMockSession(t *testing.T, sm *runner.SessionManager, sessionsDir, name string) {
	t.Helper()
	logFile := filepath.Join(sessionsDir, name+".log")
	if err := os.WriteFile(logFile, []byte("log content"), 0600); err != nil {
		t.Fatalf("Failed to create mock log file: %v", err)
	}

	session := &runner.SessionState{
		Name:      name,
		PID:       12345,
		StartTime: time.Now(),
		LogFile:   logFile,
		Status:    "completed",
	}
	if err := sm.SaveSession(session); err != nil {
		t.Fatalf("Failed to save mock session: %v", err)
	}
}

func TestDeleteCmd(t *testing.T) {
	t.Run("delete single session", func(t *testing.T) {
		sessionsDir, sm, cleanup := setupTestEnvironment(t)
		defer cleanup()
		createMockSession(t, sm, sessionsDir, "test-session-1")

		originalNewSessionManager := newDeleteSessionManager
		newDeleteSessionManager = func() (*runner.SessionManager, error) { return sm, nil }
		defer func() { newDeleteSessionManager = originalNewSessionManager }()

		output, err := executeCommand(rootCmd, "delete", "test-session-1", "--force")
		assert.NoError(t, err)
		assert.Contains(t, output, "Session 'test-session-1' deleted successfully")

		_, err = os.Stat(filepath.Join(sessionsDir, "test-session-1.json"))
		assert.True(t, os.IsNotExist(err), "session json file should be deleted")
		_, err = os.Stat(filepath.Join(sessionsDir, "test-session-1.log"))
		assert.True(t, os.IsNotExist(err), "session log file should be deleted")
	})

	t.Run("delete all sessions", func(t *testing.T) {
		sessionsDir, sm, cleanup := setupTestEnvironment(t)
		defer cleanup()
		createMockSession(t, sm, sessionsDir, "test-session-1")
		createMockSession(t, sm, sessionsDir, "test-session-2")

		originalNewSessionManager := newDeleteSessionManager
		newDeleteSessionManager = func() (*runner.SessionManager, error) { return sm, nil }
		defer func() { newDeleteSessionManager = originalNewSessionManager }()

		output, err := executeCommand(rootCmd, "delete", "--all", "--force")
		assert.NoError(t, err)
		assert.Contains(t, output, "All sessions deleted")
		assert.Contains(t, output, "Deleted session: test-session-1")
		assert.Contains(t, output, "Deleted session: test-session-2")

		_, err = os.Stat(filepath.Join(sessionsDir, "test-session-1.json"))
		assert.True(t, os.IsNotExist(err))
		_, err = os.Stat(filepath.Join(sessionsDir, "test-session-2.log"))
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("delete non-existent session", func(t *testing.T) {
		_, sm, cleanup := setupTestEnvironment(t)
		defer cleanup()

		originalNewSessionManager := newDeleteSessionManager
		newDeleteSessionManager = func() (*runner.SessionManager, error) { return sm, nil }
		defer func() { newDeleteSessionManager = originalNewSessionManager }()

		_, err := executeCommand(rootCmd, "delete", "non-existent-session", "--force")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "session 'non-existent-session' not found")
	})
}
