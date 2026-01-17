package main

import (
	"recac/internal/git"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRollbackCmd_List(t *testing.T) {
	// Setup Mocks
	mockSM := NewMockSessionManager()
	session, _ := mockSM.StartSession("test-session", "goal", []string{"cmd"}, "/tmp/workspace")
	session.PID = 0 // Not running
	session.Status = "stopped"
	mockSM.SaveSession(session) // Ensure it is in list

	mockGit := &MockGitClient{}
	mockGit.LogFunc = func(dir string, args ...string) (string, error) {
		return "abcdef1 chore: progress update (iteration 2)\n1234567 chore: progress update (iteration 1)", nil
	}

	// Override Factories
	oldSMFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) { return mockSM, nil }
	defer func() { sessionManagerFactory = oldSMFactory }()

	oldGitFactory := git.NewClient
	git.NewClient = func() git.IClient { return mockGit }
	defer func() { git.NewClient = oldGitFactory }()

	// Execute
	cmd, _, _ := newRootCmd()
	output, err := executeCommand(cmd, "rollback", "test-session", "--list")

	// Verify
	require.NoError(t, err)
	assert.Contains(t, output, "abcdef1 chore: progress update (iteration 2)")
	assert.Contains(t, output, "1234567 chore: progress update (iteration 1)")
}

func TestRollbackCmd_Exec(t *testing.T) {
	mockSM := NewMockSessionManager()
	session, _ := mockSM.StartSession("test-session", "goal", []string{"cmd"}, "/tmp/workspace")
	session.PID = 0
	session.Status = "stopped"
	mockSM.SaveSession(session)

	mockGit := &MockGitClient{}
	mockGit.LogFunc = func(dir string, args ...string) (string, error) {
		// 3 commits: 3, 2, 1.
		// rollback 1 should go to 2.
		return "aaaaaaa chore: progress update (iteration 3)\nbbbbbbb chore: progress update (iteration 2)\nccccccc chore: progress update (iteration 1)", nil
	}

	resetCalled := false
	targetSHA := ""
	mockGit.ResetFunc = func(dir, target string) error {
		resetCalled = true
		targetSHA = target
		return nil
	}

	oldSMFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) { return mockSM, nil }
	defer func() { sessionManagerFactory = oldSMFactory }()

	oldGitFactory := git.NewClient
	git.NewClient = func() git.IClient { return mockGit }
	defer func() { git.NewClient = oldGitFactory }()

	// Execute rollback 1 step
	cmd, _, _ := newRootCmd()
	output, err := executeCommand(cmd, "rollback", "test-session", "-n", "1")

	require.NoError(t, err)
	assert.True(t, resetCalled)
	assert.Equal(t, "bbbbbbb", targetSHA) // Should be the 2nd commit (index 1)
	assert.Contains(t, output, "Rolling back 1 step(s) to iteration 2 (bbbbbbb)")
}

func TestRollbackCmd_Running(t *testing.T) {
	mockSM := NewMockSessionManager()
	session, _ := mockSM.StartSession("running-session", "goal", []string{"cmd"}, "/tmp/workspace")
	session.Status = "running" // Running
	mockSM.SaveSession(session)
	// Setup PID running check
	mockSM.IsProcessRunningFunc = func(pid int) bool { return true }

	oldSMFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) { return mockSM, nil }
	defer func() { sessionManagerFactory = oldSMFactory }()

	// Execute
	cmd, _, _ := newRootCmd()
	_, err := executeCommand(cmd, "rollback", "running-session")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "session 'running-session' is currently running")
}

func TestRollbackCmd_RunningForce(t *testing.T) {
	mockSM := NewMockSessionManager()
	session, _ := mockSM.StartSession("running-session", "goal", []string{"cmd"}, "/tmp/workspace")
	session.Status = "running"
	mockSM.SaveSession(session)
	mockSM.IsProcessRunningFunc = func(pid int) bool { return true }

	mockGit := &MockGitClient{}
	mockGit.LogFunc = func(dir string, args ...string) (string, error) {
		return "aaaaaaa chore: progress update (iteration 2)\nbbbbbbb chore: progress update (iteration 1)", nil
	}
	mockGit.ResetFunc = func(dir, target string) error { return nil }

	oldSMFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) { return mockSM, nil }
	defer func() { sessionManagerFactory = oldSMFactory }()

	oldGitFactory := git.NewClient
	git.NewClient = func() git.IClient { return mockGit }
	defer func() { git.NewClient = oldGitFactory }()

	// Execute with force
	cmd, _, _ := newRootCmd()
	_, err := executeCommand(cmd, "rollback", "running-session", "--force")

	require.NoError(t, err)
}
