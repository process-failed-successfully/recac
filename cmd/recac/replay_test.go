package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"recac/internal/runner"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestEnvironment creates a temporary directory and a session manager for testing.
func setupTestEnvironment(t *testing.T) (string, *runner.SessionManager, func()) {
	tempDir, err := os.MkdirTemp("", "recac-replay-test-*")
	require.NoError(t, err)

	sm, err := runner.NewSessionManagerWithDir(tempDir)
	require.NoError(t, err)

	// Override the default session manager creation in the command
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

// createFakeSession creates a mock session file in the session manager's directory.
func createFakeSession(t *testing.T, sm *runner.SessionManager, sessionName, status string) *runner.SessionState {
	t.Helper()
	fakeSession := &runner.SessionState{
		Name:      sessionName,
		PID:       os.Getpid(),
		StartTime: time.Now().Add(-10 * time.Minute),
		Status:    status,
		LogFile:   "/tmp/test.log",
	}
	sessionPath := sm.GetSessionPath(sessionName)

	data, err := json.Marshal(fakeSession)
	if err != nil {
		t.Fatalf("Failed to marshal fake session: %v", err)
	}

	if err := os.WriteFile(sessionPath, data, 0644); err != nil {
		t.Fatalf("Failed to write fake session file: %v", err)
	}
	return fakeSession
}

func TestReplayCmd(t *testing.T) {
	_, sm, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create a fake original session
	originalSession := &runner.SessionState{
		Name:      "test-session",
		PID:       12345,
		StartTime: time.Now(),
		Command:   []string{"/bin/true"},
		Workspace: "/tmp",
		Status:    "completed",
	}
	err := sm.SaveSession(originalSession)
	require.NoError(t, err)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Execute the replay command
	rootCmd.SetArgs([]string{"replay", "test-session"})
	err = rootCmd.Execute()
	assert.NoError(t, err)

	// Restore stdout and read captured output
	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Verify the output
	assert.Contains(t, output, "Successfully started replay session")
	assert.Contains(t, output, "replay")

	// Verify that a new session was created
	sessions, err := sm.ListSessions()
	require.NoError(t, err)
	assert.Len(t, sessions, 2)

	var replayedSession *runner.SessionState
	for _, s := range sessions {
		if strings.HasPrefix(s.Name, "test-session-replay-") {
			replayedSession = s
			break
		}
	}
	require.NotNil(t, replayedSession, "replayed session not found")

	// Verify the replayed session's properties
	assert.Equal(t, originalSession.Command, replayedSession.Command)
	assert.Equal(t, originalSession.Workspace, replayedSession.Workspace)
	assert.Equal(t, "running", replayedSession.Status) // It should be running
}

func TestReplayCmd_RunningSession(t *testing.T) {
	_, sm, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create a fake running session
	runningSession := &runner.SessionState{
		Name:      "running-session",
		PID:       os.Getpid(), // Use current process's PID to simulate a running process
		StartTime: time.Now(),
		Command:   []string{"/bin/sleep", "30"},
		Workspace: "/tmp",
		Status:    "running",
	}
	err := sm.SaveSession(runningSession)
	require.NoError(t, err)

	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	// Set exit function to avoid os.Exit(1)
	oldExit := exit
	var exitCode int
	exit = func(code int) {
		exitCode = code
	}
	defer func() { exit = oldExit }()

	// Execute the command
	rootCmd.SetArgs([]string{"replay", "running-session"})
	rootCmd.Execute()

	// Restore stderr and read output
	w.Close()
	os.Stderr = oldStderr
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Verify the error message
	assert.Equal(t, 1, exitCode)
	assert.Contains(t, output, "Error: cannot replay a running session. Please stop it first.")
}
