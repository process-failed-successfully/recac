package main

import (
	"os"
	"path/filepath"
	"recac/internal/runner"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupLsTest(t *testing.T) func() {
	tempDir := t.TempDir()
	sessionDir := filepath.Join(tempDir, "sessions")
	os.MkdirAll(sessionDir, 0755)

	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return runner.NewSessionManagerWithDir(sessionDir)
	}

	return func() {
		sessionManagerFactory = originalFactory
	}
}

func TestLsCommand_NoSessions(t *testing.T) {
	teardown := setupLsTest(t)
	defer teardown()

	output, err := executeCommand(rootCmd, "ls")
	require.NoError(t, err)

	assert.Contains(t, output, "No sessions found.")
}

func TestLsCommand_WithSessions(t *testing.T) {
	teardown := setupLsTest(t)
	defer teardown()

	sm, err := sessionManagerFactory()
	require.NoError(t, err)

	session1 := &runner.SessionState{Name: "session-1", Status: "completed", StartTime: time.Now().Add(-1 * time.Hour)}
	session2 := &runner.SessionState{Name: "session-2", Status: "running", StartTime: time.Now()}
	require.NoError(t, sm.SaveSession(session1))
	require.NoError(t, sm.SaveSession(session2))

	output, err := executeCommand(rootCmd, "ls")
	require.NoError(t, err)

	assert.Contains(t, output, "SESSION_ID")
	assert.Contains(t, output, "STATUS")
	assert.Contains(t, output, "STARTED")
	assert.Contains(t, output, "DURATION")

	// The mock session manager transitions 'running' to 'completed'
	assert.Regexp(t, `session-1\s+completed`, output)
	assert.Regexp(t, `session-2\s+completed`, output)
}

func TestLsCommand_FilterByStatus(t *testing.T) {
	teardown := setupLsTest(t)
	defer teardown()

	sm, err := sessionManagerFactory()
	require.NoError(t, err)

	sessionRunning := &runner.SessionState{Name: "session-running", Status: "running", StartTime: time.Now()}
	sessionCompleted := &runner.SessionState{Name: "session-completed", Status: "completed", StartTime: time.Now()}
	sessionError := &runner.SessionState{Name: "session-error", Status: "error", StartTime: time.Now()}
	require.NoError(t, sm.SaveSession(sessionRunning))
	require.NoError(t, sm.SaveSession(sessionCompleted))
	require.NoError(t, sm.SaveSession(sessionError))

	t.Run("filter by completed", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "ls", "--status", "completed")
		require.NoError(t, err)
		assert.Contains(t, output, "session-running")   // Transitioned
		assert.Contains(t, output, "session-completed")
		assert.NotContains(t, output, "session-error")
	})

	t.Run("filter by error", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "ls", "--status", "error")
		require.NoError(t, err)
		assert.NotContains(t, output, "session-running")
		assert.NotContains(t, output, "session-completed")
		assert.Contains(t, output, "session-error")
	})

	t.Run("filter is case-insensitive", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "ls", "--status", "CoMpLeTeD")
		require.NoError(t, err)
		assert.True(t, strings.Contains(output, "session-running") || strings.Contains(output, "session-completed"))
		assert.NotContains(t, output, "session-error")
	})

	t.Run("filter with no matches", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "ls", "--status", "zombie")
		require.NoError(t, err)
		assert.Contains(t, output, "No sessions found.")
	})
}
