package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSearchLogsCommand(t *testing.T) {
	// 1. Setup: Create a temporary directory for sessions
	tmpDir := t.TempDir()
	sessionsDir := filepath.Join(tmpDir, ".recac", "sessions")
	err := os.MkdirAll(sessionsDir, 0755)
	require.NoError(t, err)

	// Set the HOME environment variable to our temp directory for the test
	t.Setenv("HOME", tmpDir)

	// 2. Create mock session files
	// Session 1: Has a match
	session1Name := "session-one"
	session1JsonPath := filepath.Join(sessionsDir, fmt.Sprintf("%s.json", session1Name))
	err = os.WriteFile(session1JsonPath, []byte(`{"name": "session-one"}`), 0644)
	require.NoError(t, err)
	session1LogPath := filepath.Join(sessionsDir, fmt.Sprintf("%s.log", session1Name))
	err = os.WriteFile(session1LogPath, []byte("INFO: Starting process\nERROR: Something went wrong\nINFO: Process finished\n"), 0644)
	require.NoError(t, err)

	// Session 2: Has two matches
	session2Name := "session-two"
	session2JsonPath := filepath.Join(sessionsDir, fmt.Sprintf("%s.json", session2Name))
	err = os.WriteFile(session2JsonPath, []byte(`{"name": "session-two"}`), 0644)
	require.NoError(t, err)
	session2LogPath := filepath.Join(sessionsDir, fmt.Sprintf("%s.log", session2Name))
	err = os.WriteFile(session2LogPath, []byte("DEBUG: Another process started\nERROR: A critical failure occurred\nWARN: Something is amiss\nERROR: Another error happened\n"), 0644)
	require.NoError(t, err)

	// Session 3: No match
	session3Name := "session-three"
	session3JsonPath := filepath.Join(sessionsDir, fmt.Sprintf("%s.json", session3Name))
	err = os.WriteFile(session3JsonPath, []byte(`{"name": "session-three"}`), 0644)
	require.NoError(t, err)
	session3LogPath := filepath.Join(sessionsDir, fmt.Sprintf("%s.log", session3Name))
	err = os.WriteFile(session3LogPath, []byte("All quiet here.\nNothing to see.\n"), 0644)
	require.NoError(t, err)

	// Session 4: No log file, should be skipped gracefully
	session4Name := "session-four-no-log"
	session4JsonPath := filepath.Join(sessionsDir, fmt.Sprintf("%s.json", session4Name))
	err = os.WriteFile(session4JsonPath, []byte(`{"name": "session-four-no-log"}`), 0644)
	require.NoError(t, err)

	t.Run("Finds matches across multiple files", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "search-logs", "ERROR")
		require.NoError(t, err)

		require.Contains(t, output, "[session-one] ERROR: Something went wrong")
		require.Contains(t, output, "[session-two] ERROR: A critical failure occurred")
		require.Contains(t, output, "[session-two] ERROR: Another error happened")
		require.NotContains(t, output, "All quiet here")
	})

	t.Run("Handles no matches", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "search-logs", "nonexistent-pattern")
		require.NoError(t, err)
		require.Contains(t, output, "No matches found.")
	})

	t.Run("Handles missing arguments", func(t *testing.T) {
		_, err = executeCommand(rootCmd, "search-logs")
		require.Error(t, err)
		require.Contains(t, err.Error(), "accepts 1 arg(s), received 0")
	})
}
