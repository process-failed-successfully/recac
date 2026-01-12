package main

import (
	"bytes"
	"fmt"
	"os"
	"recac/internal/runner"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReplayCmd(t *testing.T) {
	// 1. Setup
	originalSessionName := "original-session"
	originalWorkspace, err := os.MkdirTemp("", "replay-test-ws-")
	require.NoError(t, err)
	defer os.RemoveAll(originalWorkspace)

	// Mock the SessionManager
	mockSM := NewMockSessionManager()
	originalState := &runner.SessionState{
		Name:      originalSessionName,
		Command:   []string{"/usr/local/bin/recac", "start", "--name", originalSessionName, "--path", originalWorkspace},
		Workspace: originalWorkspace,
		Status:    "completed",
		PID:       54321,
	}
	mockSM.Sessions[originalSessionName] = originalState

	// Override the factory function to return our mock
	originalNewSessionManager := newSessionManager
	newSessionManager = func() (ISessionManager, error) {
		return mockSM, nil
	}
	defer func() { newSessionManager = originalNewSessionManager }()

	// 2. Execute the command
	rootCmd, _, _ := newRootCmd()
	var outBuf bytes.Buffer
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&outBuf)
	rootCmd.SetArgs([]string{"replay", originalSessionName})
	err = rootCmd.Execute()

	// 3. Assertions
	require.NoError(t, err)

	// Check that a new session was created with the correct naming convention
	replayedSessionName := "original-session-replay-1"
	replayedSession, exists := mockSM.Sessions[replayedSessionName]
	require.True(t, exists, "Expected replayed session to be created")

	// Verify the replayed session's configuration
	assert.Equal(t, replayedSessionName, replayedSession.Name)
	assert.Equal(t, originalWorkspace, replayedSession.Workspace)

	// Verify the command was correctly modified
	foundNameFlag := false
	for i, arg := range replayedSession.Command {
		if arg == "--name" {
			require.True(t, i+1 < len(replayedSession.Command), "Command should have a value for --name")
			assert.Equal(t, replayedSessionName, replayedSession.Command[i+1])
			foundNameFlag = true
		}
	}
	require.True(t, foundNameFlag, "Expected --name flag to be present in the replayed command")

	// Verify the executable path was updated (os.Executable() will be the test binary)
	executable, _ := os.Executable()
	assert.Equal(t, executable, replayedSession.Command[0])

	// Check the output message
	assert.Contains(t, outBuf.String(), fmt.Sprintf("Successfully replayed session '%s' as '%s'", originalSessionName, replayedSessionName))
}

func TestReplayCmd_NoOriginalSession(t *testing.T) {
	// Setup a mock manager with no sessions
	mockSM := NewMockSessionManager()
	originalNewSessionManager := newSessionManager
	newSessionManager = func() (ISessionManager, error) {
		return mockSM, nil
	}
	defer func() { newSessionManager = originalNewSessionManager }()

	// Execute
	rootCmd, _, _ := newRootCmd()
	var errBuf bytes.Buffer
	rootCmd.SetOut(&errBuf)
	rootCmd.SetErr(&errBuf)
	rootCmd.SetArgs([]string{"replay", "non-existent-session"})
	err := rootCmd.Execute()

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load session 'non-existent-session'")
}
