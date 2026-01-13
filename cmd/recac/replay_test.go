package main

import (
	"os"
	"recac/internal/runner"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReplayCmd(t *testing.T) {
	// Setup: Create a mock session manager and a completed session
	mockSM := NewMockSessionManager()
	mockSM.IsProcessRunningFunc = func(pid int) bool {
		return false // Assume process is not running for completed sessions
	}

	originalSession := &runner.SessionState{
		Name:           "test-session",
		Command:        []string{"/bin/echo", "hello"},
		Workspace:      "/tmp/workspace",
		Status:         "completed",
		StartCommitSHA: "abcdef123",
	}
	mockSM.Sessions["test-session"] = originalSession

	// Setup: Create a mock git client
	mockGit := &MockGitClient{
		CheckoutFunc: func(repoPath, commitOrBranch string) error {
			assert.Equal(t, originalSession.Workspace, repoPath)
			assert.Equal(t, originalSession.StartCommitSHA, commitOrBranch)
			return nil
		},
	}

	// Override factories
	originalSMFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return mockSM, nil
	}
	defer func() { sessionManagerFactory = originalSMFactory }()

	originalGitFactory := gitClientFactory
	gitClientFactory = func() IGitClient {
		return mockGit
	}
	defer func() { gitClientFactory = originalGitFactory }()

	// Execute the command
	cmd, _, _ := newRootCmd()
	output, err := executeCommand(cmd, "replay", "test-session")

	// Assertions
	require.NoError(t, err)
	assert.Contains(t, output, "Restoring workspace to original commit: abcdef123")
	assert.Contains(t, output, "Successfully started replay session 'test-session-replay-1'")

	// Verify a new session was started
	require.Contains(t, mockSM.Sessions, "test-session-replay-1")
	replayedSession := mockSM.Sessions["test-session-replay-1"]
	assert.Equal(t, originalSession.Command, replayedSession.Command)
	assert.Equal(t, originalSession.Workspace, replayedSession.Workspace)
}

func TestReplayCmd_RunningSession(t *testing.T) {
	// Setup: Create a mock session manager with a running session
	mockSM := NewMockSessionManager()
	mockSM.IsProcessRunningFunc = func(pid int) bool {
		return pid == 12345 // This is the running process
	}
	runningSession := &runner.SessionState{
		Name:    "running-session",
		PID:     12345,
		Status:  "running",
		Command: []string{"/bin/sleep", "60"},
	}
	mockSM.Sessions["running-session"] = runningSession

	// Override factory
	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return mockSM, nil
	}
	defer func() { sessionManagerFactory = originalFactory }()

	// Execute and assert error
	cmd, _, _ := newRootCmd()
	_, err := executeCommand(cmd, "replay", "running-session")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot replay a running session")
}

func TestReplayCmd_SessionNotFound(t *testing.T) {
	// Setup: Empty mock session manager
	mockSM := &MockSessionManager{
		Sessions: make(map[string]*runner.SessionState),
	}
	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return mockSM, nil
	}
	defer func() { sessionManagerFactory = originalFactory }()

	// Execute and assert error
	cmd, _, _ := newRootCmd()
	_, err := executeCommand(cmd, "replay", "non-existent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load session 'non-existent'")
}

func TestFindNextReplayName(t *testing.T) {
	tests := []struct {
		name           string
		baseName       string
		existing       []string
		expectedName   string
		expectErr      bool
	}{
		{
			name:         "first replay",
			baseName:     "session-a",
			existing:     []string{"session-a", "session-b"},
			expectedName: "session-a-replay-1",
		},
		{
			name:         "second replay",
			baseName:     "session-a",
			existing:     []string{"session-a", "session-a-replay-1"},
			expectedName: "session-a-replay-2",
		},
		{
			name:         "replay a replay",
			baseName:     "session-a-replay-1",
			existing:     []string{"session-a", "session-a-replay-1"},
			expectedName: "session-a-replay-1-replay-1",
		},
		{
			name:         "gaps in numbering",
			baseName:     "session-c",
			existing:     []string{"session-c", "session-c-replay-1", "session-c-replay-3"},
			expectedName: "session-c-replay-4",
		},
		{
			name:         "no existing sessions",
			baseName:     "session-d",
			existing:     []string{},
			expectedName: "session-d-replay-1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockSM := &MockSessionManager{Sessions: make(map[string]*runner.SessionState)}
			for _, name := range tc.existing {
				mockSM.Sessions[name] = &runner.SessionState{Name: name}
			}

			actualName, err := findNextReplayName(mockSM, tc.baseName)

			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expectedName, actualName)
			}
		})
	}
}


// This test setup is now simplified by using the shared helpers.
// A mock implementation of ISessionManager is defined in test_helpers_test.go.

func setupRealSM(t *testing.T) (string, ISessionManager, func()) {
	dir, err := os.MkdirTemp("", "replay-test-")
	require.NoError(t, err)
	sm, err := runner.NewSessionManagerWithDir(dir)
	require.NoError(t, err)

	cleanup := func() {
		os.RemoveAll(dir)
	}

	return dir, sm, cleanup
}
