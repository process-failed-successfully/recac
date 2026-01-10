package main

import (
	"os"
	"path/filepath"
	"recac/internal/runner"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSearchCodeCmd(t *testing.T) {
	tmpDir := t.TempDir()
	sessionsDir := filepath.Join(tmpDir, "sessions")
	require.NoError(t, os.MkdirAll(sessionsDir, 0755))

	// Create a session manager that points to our temporary directory
	sm, err := runner.NewSessionManagerWithDir(tmpDir)
	require.NoError(t, err)

	// Override the global factory to inject our test-specific session manager
	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		// We return the real session manager here, but the mock will be used for assertions
		// In a real-world scenario, you would inject the mock here.
		// For this test, we are relying on the fact that the command uses the factory
		// to create the session manager, and we can inspect the mock's state.
		// This is a bit of a hack, but it works for this test.
		return sm, nil
	}
	defer func() { sessionManagerFactory = originalFactory }()

	// --- Setup mock session workspaces ---

	// Session 1: Has a matching file in the workspace root
	session1Name := "session-with-match"
	session1Workspace := filepath.Join(sessionsDir, session1Name, "workspace")
	require.NoError(t, os.MkdirAll(session1Workspace, 0755))
	err = os.WriteFile(filepath.Join(session1Workspace, "main.go"), []byte("package main\n\nfunc main() {\n\tprintln(\"hello from session 1\")\n}\n"), 0644)
	require.NoError(t, err)

	// Session 2: Has a matching file in a subdirectory and a non-matching file
	session2Name := "session-with-subdir"
	session2WorkspaceSubdir := filepath.Join(sessionsDir, session2Name, "workspace", "api")
	require.NoError(t, os.MkdirAll(session2WorkspaceSubdir, 0755))
	err = os.WriteFile(filepath.Join(session2WorkspaceSubdir, "handler.go"), []byte("package api\n\n// hello in a comment\n"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(sessionsDir, session2Name, "workspace", "config.json"), []byte(`{"key": "value"}`), 0644)
	require.NoError(t, err)

	// Session 3: Has a file in an ignored directory (.git) that should not be searched
	session3Name := "session-with-ignored-dir"
	session3IgnoredDir := filepath.Join(sessionsDir, session3Name, "workspace", ".git")
	require.NoError(t, os.MkdirAll(session3IgnoredDir, 0755))
	err = os.WriteFile(filepath.Join(session3IgnoredDir, "config"), []byte("should not say hello"), 0644)
	require.NoError(t, err)

	// Session 4: Has no workspace directory, should be skipped
	session4Name := "session-no-workspace"
	require.NoError(t, os.MkdirAll(filepath.Join(sessionsDir, session4Name), 0755))

	// Create dummy session state files so the manager can find them
	for _, name := range []string{session1Name, session2Name, session3Name, session4Name} {
		state := &runner.SessionState{Name: name, Status: "COMPLETED"}
		err := sm.SaveSession(state)
		require.NoError(t, err)
	}

	// --- Run tests ---

	t.Run("finds matches in root and subdirectories", func(t *testing.T) {
		rootCmd, _, _ := newRootCmd()
		output, err := executeCommand(rootCmd, "search-code", "hello")
		require.NoError(t, err)

		// Check for match in session 1
		require.Contains(t, output, "[session-with-match:main.go:4] \tprintln(\"hello from session 1\")")
		// Check for match in session 2
		require.Contains(t, output, "[session-with-subdir:api/handler.go:3] // hello in a comment")
		// Ensure ignored file content is not present
		require.NotContains(t, output, "should not say hello")
		// Ensure it doesn't report "no matches"
		require.NotContains(t, output, "No matches found.")
	})

	t.Run("reports no matches when pattern is not found", func(t *testing.T) {
		rootCmd, _, _ := newRootCmd()
		output, err := executeCommand(rootCmd, "search-code", "nonexistent-pattern-xyz")
		require.NoError(t, err)
		require.Contains(t, output, "No matches found.")
	})
}
