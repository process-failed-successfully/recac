package main

import (
	"recac/internal/runner"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestShowCmd(t *testing.T) {
	// 1. Setup Mock Session Manager
	mockSM := NewMockSessionManager()
	session := &runner.SessionState{
		Name:           "test-session",
		Status:         "completed",
		StartCommitSHA: "start-sha",
		EndCommitSHA:   "end-sha",
		Workspace:      "/tmp/recac-test",
	}
	mockSM.Sessions[session.Name] = session

	// 2. Setup Mock Git Client
	mockGit := &MockGitClient{
		DiffFunc: func(workspace, fromSHA, toSHA string) (string, error) {
			require.Equal(t, "/tmp/recac-test", workspace)
			require.Equal(t, "start-sha", fromSHA)
			require.Equal(t, "end-sha", toSHA)
			return "--- a/file.txt\n+++ b/file.txt\n@@ -1 +1 @@\n-hello\n+world", nil
		},
	}

	// 3. Override factories to inject mocks
	originalSMFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return mockSM, nil
	}
	defer func() { sessionManagerFactory = originalSMFactory }()

	originalGitFactory := gitNewClient
	gitNewClient = func() gitClient {
		return mockGit
	}
	defer func() { gitNewClient = originalGitFactory }()

	t.Run("shows diff for a valid session", func(t *testing.T) {
		// 4. Execute Command
		output, err := executeCommand(rootCmd, "show", "test-session")

		// 5. Assert Output
		require.NoError(t, err)
		expectedDiff := "--- a/file.txt\n+++ b/file.txt\n@@ -1 +1 @@\n-hello\n+world\n"
		require.Equal(t, expectedDiff, output)
	})

	t.Run("handles session not found", func(t *testing.T) {
		_, err := executeCommand(rootCmd, "show", "non-existent-session")
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to load session non-existent-session")
	})

	t.Run("handles session with no start commit", func(t *testing.T) {
		// Setup session without start SHA
		noShaSession := &runner.SessionState{Name: "no-sha-session"}
		mockSM.Sessions[noShaSession.Name] = noShaSession

		_, err := executeCommand(rootCmd, "show", "no-sha-session")
		require.Error(t, err)
		require.Contains(t, err.Error(), "does not have a start commit SHA recorded")
	})
}
