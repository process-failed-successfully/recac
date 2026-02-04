package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestShowAliasCmd(t *testing.T) {
	sm, sessionName, repoDir := setupWorkdiffTest(t)
	defer os.RemoveAll(repoDir)

	rootCmd, _, _ := newRootCmd()
	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return sm, nil
	}
	defer func() { sessionManagerFactory = originalFactory }()

	t.Run("show with one argument should succeed", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "show", sessionName)
		require.NoError(t, err)
		require.Contains(t, output, "diff --git a/test.txt b/test.txt")
		require.Contains(t, output, "-hello")
		require.Contains(t, output, "+hello world")
	})

	t.Run("show with two arguments should fail", func(t *testing.T) {
		_, err := executeCommand(rootCmd, "show", sessionName, "another-session")
		require.Error(t, err)
		require.Equal(t, "the 'show' alias requires exactly one session name", err.Error())
	})

	t.Run("show with no arguments should fail", func(t *testing.T) {
		_, err := executeCommand(rootCmd, "show")
		require.Error(t, err)
		require.Equal(t, "the 'show' alias requires exactly one session name", err.Error())
	})
}
