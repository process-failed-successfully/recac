package main

import (
	"fmt"
	"recac/internal/runner"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReplayCmd(t *testing.T) {
	// 1. Setup
	originalSessionName := "test-session-original"
	replayedSessionName := "test-session-original-replay-20230101-120000"

	mockSm := &MockSessionManager{
		Sessions: make(map[string]*runner.SessionState),
		ReplaySessionFunc: func(name string) (*runner.SessionState, error) {
			if name != originalSessionName {
				return nil, fmt.Errorf("session not found")
			}
			return &runner.SessionState{
				Name:    replayedSessionName,
				PID:     12345,
				LogFile: "/tmp/replay.log",
			}, nil
		},
	}

	// Override the factory
	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return mockSm, nil
	}
	defer func() { sessionManagerFactory = originalFactory }()

	// 2. Execute
	rootCmd, _, _ := newRootCmd()
	output, err := executeCommand(rootCmd, "replay", originalSessionName)

	// 3. Assert
	require.NoError(t, err)
	assert.Contains(t, output, fmt.Sprintf("Successfully started replayed session '%s' (PID: 12345)", replayedSessionName))
	assert.Contains(t, output, "Logs are available at: /tmp/replay.log")

	// 4. Verify mock calls
	require.True(t, mockSm.ReplaySessionCalled, "ReplaySession was not called")
	require.Equal(t, originalSessionName, mockSm.ReplaySessionArg, "ReplaySession was called with the wrong argument")
}

func TestReplayCmd_NotFound(t *testing.T) {
	// 1. Setup
	nonExistentSession := "non-existent-session"

	mockSm := &MockSessionManager{
		Sessions: make(map[string]*runner.SessionState),
		ReplaySessionFunc: func(name string) (*runner.SessionState, error) {
			return nil, fmt.Errorf("session '%s' not found", name)
		},
	}

	// Override the factory
	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return mockSm, nil
	}
	defer func() { sessionManagerFactory = originalFactory }()

	// 2. Execute
	rootCmd, _, _ := newRootCmd()
	_, err := executeCommand(rootCmd, "replay", nonExistentSession)

	// 3. Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("failed to replay session: session '%s' not found", nonExistentSession))

	// 4. Verify mock calls
	require.True(t, mockSm.ReplaySessionCalled, "ReplaySession was not called")
}
