package main

import (
	"os"
	"testing"

	"recac/internal/runner"

	"github.com/stretchr/testify/assert"
)

// A minimal setupTestEnvironment, similar to the one deleted, to support the test.
func setupTestEnvironment(t *testing.T) (string, *runner.SessionManager, func()) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "recac-replay-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	sm, err := runner.NewSessionManagerWithDir(tempDir)
	if err != nil {
		t.Fatalf("Failed to create session manager: %v", err)
	}

	originalNewSessionManager := newSessionManager
	newSessionManager = func() (*runner.SessionManager, error) {
		return sm, nil
	}

	cleanup := func() {
		os.RemoveAll(tempDir)
		newSessionManager = originalNewSessionManager
	}

	return tempDir, sm, cleanup
}

func TestReplayCommand(t *testing.T) {
	// We need to manage session state. A real SM in a temp dir is the way to go.
	_, sm, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Case 1: Successful replay
	t.Run("Success", func(t *testing.T) {
		originalSession := &runner.SessionState{
			Name:      "test-success",
			Status:    "completed",
			Command:   []string{os.Args[0], "-test.run=^$"}, // Use the test binary itself as a valid command
			Workspace: t.TempDir(),
		}
		err := sm.SaveSession(originalSession)
		assert.NoError(t, err)

		output, err := executeCommand(rootCmd, "replay", "test-success")
		assert.NoError(t, err)
		assert.Contains(t, output, "Successfully started replay session")
	})

	// Case 2: Session not found
	t.Run("Not Found", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "replay", "test-not-found")
		assert.Error(t, err)
		assert.Contains(t, output, "Error: failed to load session 'test-not-found'")
	})

	// Case 3: Replaying a running session
	t.Run("Running Session", func(t *testing.T) {
		runningSession := &runner.SessionState{
			Name:      "test-running",
			Status:    "running",
			PID:       os.Getpid(), // Simulate running process
			Command:   []string{"sleep", "10"},
			Workspace: t.TempDir(),
		}
		err := sm.SaveSession(runningSession)
		assert.NoError(t, err)

		output, err := executeCommand(rootCmd, "replay", "test-running")
		assert.Error(t, err)
		assert.Contains(t, output, "Error: cannot replay a running session")
	})
}
