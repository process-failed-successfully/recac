package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestShowCmd(t *testing.T) {
	sm, sessionName, repoDir := setupWorkdiffTest(t)
	defer os.RemoveAll(repoDir)

	// Execute the command
	rootCmd, _, _ := newRootCmd()
	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return sm, nil
	}
	defer func() { sessionManagerFactory = originalFactory }()

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs([]string{"show", sessionName})

	err := rootCmd.Execute()
	require.NoError(t, err)

	// Verify the output
	actualOutput := out.String()
	require.Contains(t, actualOutput, "diff --git a/test.txt b/test.txt")
	require.Contains(t, actualOutput, "-hello")
	require.Contains(t, actualOutput, "+hello world")
}
