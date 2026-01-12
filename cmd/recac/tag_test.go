package main

import (
	"os"
	"recac/internal/runner"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTagCommand(t *testing.T) {
	// Setup
	dir, err := os.MkdirTemp("", "recac-test-sessions")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	oldSessionManager := sessionManagerFactory
	sm, err := runner.NewSessionManagerWithDir(dir)
	require.NoError(t, err)
	sessionManagerFactory = func() (ISessionManager, error) {
		return sm, nil
	}
	defer func() { sessionManagerFactory = oldSessionManager }()

	// Create mock sessions
	mockSession1 := &runner.SessionState{Name: "session-1", Status: "completed", StartTime: time.Now()}
	mockSession2 := &runner.SessionState{Name: "session-2", Status: "completed", StartTime: time.Now(), Tags: []string{"initial-tag"}}
	require.NoError(t, sm.SaveSession(mockSession1))
	require.NoError(t, sm.SaveSession(mockSession2))

	rootCmd, _, _ := newRootCmd()

	t.Run("add tag to session", func(t *testing.T) {
		rootCmd.SetArgs([]string{"tag", "add", "bugfix", "session-1"})
		err := rootCmd.Execute()
		require.NoError(t, err)

		s, err := sm.LoadSession("session-1")
		require.NoError(t, err)
		require.Equal(t, []string{"bugfix"}, s.Tags)
	})

	t.Run("add multiple tags to multiple sessions", func(t *testing.T) {
		rootCmd.SetArgs([]string{"tag", "add", "frontend", "session-1", "session-2"})
		err := rootCmd.Execute()
		require.NoError(t, err)

		s1, err := sm.LoadSession("session-1")
		require.NoError(t, err)
		require.ElementsMatch(t, []string{"bugfix", "frontend"}, s1.Tags)

		s2, err := sm.LoadSession("session-2")
		require.NoError(t, err)
		require.ElementsMatch(t, []string{"initial-tag", "frontend"}, s2.Tags)
	})

	t.Run("add duplicate tag", func(t *testing.T) {
		rootCmd.SetArgs([]string{"tag", "add", "bugfix", "session-1"})
		err := rootCmd.Execute()
		require.NoError(t, err)

		s, err := sm.LoadSession("session-1")
		require.NoError(t, err)
		require.Equal(t, []string{"bugfix", "frontend"}, s.Tags) // Should not add a duplicate
	})

	t.Run("remove tag from session", func(t *testing.T) {
		rootCmd.SetArgs([]string{"tag", "remove", "bugfix", "session-1"})
		err := rootCmd.Execute()
		require.NoError(t, err)

		s, err := sm.LoadSession("session-1")
		require.NoError(t, err)
		require.Equal(t, []string{"frontend"}, s.Tags)
	})

	t.Run("remove non-existent tag", func(t *testing.T) {
		rootCmd.SetArgs([]string{"tag", "remove", "non-existent", "session-1"})
		err := rootCmd.Execute()
		require.NoError(t, err)

		s, err := sm.LoadSession("session-1")
		require.NoError(t, err)
		require.Equal(t, []string{"frontend"}, s.Tags) // Tags should be unchanged
	})

	t.Run("remove tag from multiple sessions", func(t *testing.T) {
		rootCmd.SetArgs([]string{"tag", "remove", "frontend", "session-1", "session-2"})
		err := rootCmd.Execute()
		require.NoError(t, err)

		s1, err := sm.LoadSession("session-1")
		require.NoError(t, err)
		require.Empty(t, s1.Tags)

		s2, err := sm.LoadSession("session-2")
		require.NoError(t, err)
		require.Equal(t, []string{"initial-tag"}, s2.Tags)
	})
}
