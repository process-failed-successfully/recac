package main

import (
	"os"
	"path/filepath"
	"recac/internal/runner"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArchiveCmd(t *testing.T) {
	// Setup a temporary session manager and a mock session
	sm, cleanup := setupTestSessionManager(t)
	defer cleanup()

	sessionName := "test-archive-cmd"
	session := &runner.SessionState{Name: sessionName, Status: "completed", LogFile: filepath.Join(sm.SessionsDir(), sessionName+".log")}
	err := sm.SaveSession(session)
	require.NoError(t, err)
	_, err = os.Create(session.LogFile)
	require.NoError(t, err)

	// Execute the archive command
	rootCmd, out, _ := newRootCmd()
	rootCmd.SetArgs([]string{"archive", sessionName})
	err = rootCmd.Execute()
	require.NoError(t, err)

	// Verify the output and that the session was archived
	assert.Contains(t, out.String(), "Archived session 'test-archive-cmd'")

	_, err = sm.LoadSession(sessionName)
	assert.Error(t, err, "Session should not be in active list")

	archived, err := sm.ListArchivedSessions()
	require.NoError(t, err)
	require.Len(t, archived, 1)
	assert.Equal(t, sessionName, archived[0].Name)
}
