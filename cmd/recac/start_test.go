package main

import (
	"os"
	"path/filepath"
	"recac/internal/runner"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStartCommandWithTags(t *testing.T) {
	// Setup
	dir, err := os.MkdirTemp("", "recac-test-sessions")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	// Create a dummy executable
	dummyExec, err := os.Create(filepath.Join(dir, "dummy_exec"))
	require.NoError(t, err)
	dummyExec.Close()
	err = os.Chmod(dummyExec.Name(), 0755)
	require.NoError(t, err)

	oldSessionManager := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return runner.NewSessionManagerWithDir(dir)
	}
	defer func() { sessionManagerFactory = oldSessionManager }()

	sm, err := sessionManagerFactory()
	require.NoError(t, err)

	rootCmd, _, _ := newRootCmd()
	rootCmd.SetArgs([]string{"start", "--detached", "--name", "test-session-with-tags", "--tag", "backend", "--tag", "feature", "--", dummyExec.Name()})
	err = rootCmd.Execute()
	require.NoError(t, err)

	// Verify session was created with tags
	loadedSession, err := sm.LoadSession("test-session-with-tags")
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"backend", "feature"}, loadedSession.Tags)
}
