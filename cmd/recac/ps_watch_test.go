package main

import (
	"recac/internal/runner"
	"recac/internal/ui"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPsCommandWatchMode(t *testing.T) {
	// --- Setup ---
	sm, cleanup := setupTestSessionManager(t)
	defer cleanup()

	// Ensure ui.GetSessions is reset after the test to prevent state leakage
	defer func() { ui.GetSessions = nil }()

	// Create mock sessions. The "running" session will be transitioned to "completed"
	// by the SessionManager because its PID (0) is not active.
	sessionRunning := &runner.SessionState{Name: "session-running-now-completed", Status: "running", StartTime: time.Now()}
	sessionCompleted := &runner.SessionState{Name: "session-completed", Status: "completed", StartTime: time.Now()}
	require.NoError(t, sm.SaveSession(sessionRunning))
	require.NoError(t, sm.SaveSession(sessionCompleted))

	// Monkey-patch the TUI starter to avoid running the actual UI
	var dashboardStarted bool
	originalStartDashboard := ui.StartPsDashboard
	ui.StartPsDashboard = func(showCosts bool) error {
		dashboardStarted = true
		return nil
	}
	defer func() { ui.StartPsDashboard = originalStartDashboard }()

	// --- Execution ---
	// Execute the command with --watch. The --status filter will be applied
	// by the GetSessions func.
	cmd, _, _ := newRootCmd()
	_, err := executeCommand(cmd, "ps", "--watch", "--status", "completed")
	require.NoError(t, err)

	// --- Assertions ---
	// 1. Verify the dashboard was supposed to start
	assert.True(t, dashboardStarted, "ui.StartPsDashboard should have been called")

	// 2. Verify the GetSessions function was set by the command
	require.NotNil(t, ui.GetSessions, "ui.GetSessions should have been set")

	// 3. Call the injected function to test the data it provides
	sessions, err := ui.GetSessions()
	require.NoError(t, err)

	// 4. Check the filtering logic, expecting 2 sessions because the 'running' one was transitioned.
	assert.Len(t, sessions, 2, "should have found two sessions with status 'completed'")

	// 5. Verify that our original 'completed' session is present in the results.
	var foundOriginalCompleted bool
	for _, s := range sessions {
		if s.Name == "session-completed" {
			foundOriginalCompleted = true
			break
		}
	}
	assert.True(t, foundOriginalCompleted, "'session-completed' should be in the results")
}
