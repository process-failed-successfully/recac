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
	archivedSession := &runner.SessionState{Name: archivedSessionName, Status: "completed"}

	// Manually create an archived session file for the test
	archivedSessionPath := filepath.Join(sm.SessionsDir(), "archived", archivedSessionName+".json")
	archivedLogPath := filepath.Join(sm.SessionsDir(), "archived", archivedSessionName+".log")

	err := sm.SaveSession(archivedSession)
	require.NoError(t, err)
	// Now, archive it by moving it manually for the test setup
	err = os.Rename(sm.GetSessionPath(archivedSessionName), archivedSessionPath)
	require.NoError(t, err)
	_, err = os.Create(archivedLogPath) // Create a dummy log file
	require.NoError(t, err)

	// Execute the unarchive command
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
