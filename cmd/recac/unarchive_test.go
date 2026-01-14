package main

import (
	"os"
	"path/filepath"
	"recac/internal/runner"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnarchiveCmd(t *testing.T) {
	// Setup a temporary session manager and a mock archived session
	sm, cleanup := setupTestSessionManager(t)
	defer cleanup()

	archivedSessionName := "test-unarchive-cmd"
	// Create a session and then archive it to set up the test state properly.
	sessionToArchive := &runner.SessionState{
		Name:    archivedSessionName,
		Status:  "completed",
		LogFile: filepath.Join(sm.SessionsDir(), archivedSessionName+".log"),
	}
	err := sm.SaveSession(sessionToArchive)
	require.NoError(t, err)
	_, err = os.Create(sessionToArchive.LogFile) // Create a dummy log file
	require.NoError(t, err)

	// Use the application's own logic to archive the session.
	err = sm.ArchiveSession(archivedSessionName)
	require.NoError(t, err, "Setup: failed to archive session")

	// Execute the unarchive command.
	rootCmd, out, _ := newRootCmd()
	rootCmd.SetArgs([]string{"unarchive", archivedSessionName})
	err = rootCmd.Execute()
	require.NoError(t, err)

	// Verify the output and that the session was unarchived
	assert.Contains(t, out.String(), "Unarchived session 'test-unarchive-cmd'")

	_, err = sm.LoadSession(archivedSessionName)
	assert.NoError(t, err, "Session should be in active list")

	archived, err := sm.ListArchivedSessions()
	require.NoError(t, err)
	assert.Len(t, archived, 0)
}
