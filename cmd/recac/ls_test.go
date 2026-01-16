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

	session1 := &runner.SessionState{Name: "session-1", Status: "completed", StartTime: time.Now().Add(-1 * time.Hour), Goal: "Test Goal 1"}
	session2 := &runner.SessionState{Name: "session-2", Status: "running", StartTime: time.Now(), Goal: "Test Goal 2"}
	require.NoError(t, sm.SaveSession(session1))
	require.NoError(t, sm.SaveSession(session2))

	output, err := executeCommand(rootCmd, "ls")
	require.NoError(t, err)

	assert.Contains(t, output, "SESSION_ID")
	assert.Contains(t, output, "GOAL")
	assert.Contains(t, output, "STATUS")
	assert.Contains(t, output, "STARTED")
	assert.Contains(t, output, "DURATION")

	// The mock session manager transitions 'running' to 'completed'
	assert.Regexp(t, `session-1\s+Test Goal 1\s+completed`, output)
	assert.Regexp(t, `session-2\s+Test Goal 2\s+completed`, output)
}

func TestLsCommand_WithGoal(t *testing.T) {
	teardown := setupLsTest(t)
	defer teardown()

	sm, err := sessionManagerFactory()
	require.NoError(t, err)

	session1 := &runner.SessionState{Name: "session-1", Status: "completed", StartTime: time.Now().Add(-1 * time.Hour), Goal: "Test Goal"}
	require.NoError(t, sm.SaveSession(session1))

	output, err := executeCommand(rootCmd, "ls")
	require.NoError(t, err)

	assert.Contains(t, output, "SESSION_ID")
	assert.Contains(t, output, "GOAL")
	assert.Contains(t, output, "STATUS")
	assert.Contains(t, output, "STARTED")
	assert.Contains(t, output, "DURATION")

	assert.Regexp(t, `session-1\s+Test Goal\s+completed`, output)
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

func TestLsCommand_FilterByTime(t *testing.T) {
	teardown := setupLsTest(t)
	defer teardown()

	sm, err := sessionManagerFactory()
	require.NoError(t, err)

	now := time.Now()
	sessionOld := &runner.SessionState{Name: "session-old", Status: "completed", StartTime: now.Add(-10 * 24 * time.Hour)}
	sessionRecent := &runner.SessionState{Name: "session-recent", Status: "completed", StartTime: now.Add(-1 * time.Hour)}
	sessionNew := &runner.SessionState{Name: "session-new", Status: "running", StartTime: now.Add(-1 * time.Minute)}

	require.NoError(t, sm.SaveSession(sessionOld))
	require.NoError(t, sm.SaveSession(sessionRecent))
	require.NoError(t, sm.SaveSession(sessionNew))

	t.Run("filter with --since duration", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "ls", "--since", "2h")
		require.NoError(t, err)

		assert.NotContains(t, output, "session-old")
		assert.Contains(t, output, "session-recent")
		assert.Contains(t, output, "session-new")
	})

	t.Run("filter with --since timestamp", func(t *testing.T) {
		sinceTime := now.Add(-3 * time.Hour).Format("2006-01-02")
		output, err := executeCommand(rootCmd, "ls", "--since", sinceTime)
		require.NoError(t, err)

		assert.NotContains(t, output, "session-old")
		assert.Contains(t, output, "session-recent")
		assert.Contains(t, output, "session-new")
	})

	t.Run("filter with --before duration", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "ls", "--before", "5h")
		require.NoError(t, err)

		assert.Contains(t, output, "session-old")
		assert.NotContains(t, output, "session-recent")
		assert.NotContains(t, output, "session-new")
	})

	t.Run("filter with --before timestamp", func(t *testing.T) {
		beforeTime := now.Add(-5 * time.Hour).Format("2006-01-02T15:04:05Z07:00")
		output, err := executeCommand(rootCmd, "ls", "--before", beforeTime)
		require.NoError(t, err)

		assert.Contains(t, output, "session-old")
		assert.NotContains(t, output, "session-recent")
		assert.NotContains(t, output, "session-new")
	})

	t.Run("filter with --since and --before", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "ls", "--since", "5d", "--before", "30m")
		require.NoError(t, err)

		assert.NotContains(t, output, "session-old")
		assert.Contains(t, output, "session-recent")
		assert.NotContains(t, output, "session-new")
	})

	t.Run("filter with invalid time value", func(t *testing.T) {
		_, err := executeCommand(rootCmd, "ls", "--since", "invalid-time")
		require.Error(t, err)
		assert.Contains(t, err.Error(), `invalid time value "invalid-time"`)
	})

	t.Run("combined filter with --status and --since", func(t *testing.T) {
		// Note: The mock session manager transitions 'running' status to 'completed' upon listing.
		// So we filter for 'completed' to find the 'running' session.
		output, err := executeCommand(rootCmd, "ls", "--status", "completed", "--since", "2h")
		require.NoError(t, err)

		assert.NotContains(t, output, "session-old")
		assert.Contains(t, output, "session-recent") // Is completed and recent
		assert.Contains(t, output, "session-new")    // Was running, now completed and recent
	})
}
