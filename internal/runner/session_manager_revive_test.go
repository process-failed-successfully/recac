package runner

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionManager_ReviveSession(t *testing.T) {
	sm, cleanup := setupSessionManager(t)
	defer cleanup()

	// Use sh to echo and sleep
	shCmd, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh command not found, skipping TestSessionManager_ReviveSession")
	}

	// 1. Start a session
	sessionName := "test-revive-session"
	// Command: sh -c "echo run1; sleep 0.1"
	command := []string{shCmd, "-c", "echo run1; sleep 0.1"}
	session, err := sm.StartSession(sessionName, "test goal", command, sm.SessionsDir())
	require.NoError(t, err, "Failed to start session")
	require.Equal(t, "running", session.Status, "Session should be running")

	// 2. Wait for it to complete
	// We need to ensure the process is reaped so IsProcessRunning returns false
	// Since we don't have the cmd handle, we just wait enough time and hope the OS or test runner doesn't zombie it too hard.
	// But wait, IsProcessRunning checks signal 0. Zombies respond to signal 0.
	// The only way to reap it is if the parent (us) waits for it.
	// But we started it via StartSession which starts it detached (no Wait call).
	// So it IS a zombie.

	// However, ResumeSession (my new code) checks IsProcessRunning.
	// If IsProcessRunning returns true (zombie), ResumeSession will think it is running and fail!

	// I need to manually reap the process in the test using the PID.
	time.Sleep(500 * time.Millisecond) // Wait for sleep to finish

	proc, err := os.FindProcess(session.PID)
	if err == nil {
		proc.Wait() // Reap zombie
	}

	// Refresh session status (ListSessions updates completed sessions)
	sessions, err := sm.ListSessions()
	require.NoError(t, err)
	var loaded *SessionState
	for _, s := range sessions {
		if s.Name == sessionName {
			loaded = s
			break
		}
	}
	require.NotNil(t, loaded, "Session should be found")

	// ListSessions might have updated it to completed if IsProcessRunning returned false
	// Now that we reaped it, IsProcessRunning should return false.

	// Refresh again to trigger update
	sessions, _ = sm.ListSessions()
	for _, s := range sessions {
		if s.Name == sessionName {
			loaded = s
			break
		}
	}
	require.Equal(t, "completed", loaded.Status, "Session should be completed")

	// 3. Revive the session
	err = sm.ResumeSession(sessionName)
	assert.NoError(t, err, "ResumeSession should succeed (revive)")

	// 4. Verify it is running again
	revivedSession, err := sm.LoadSession(sessionName)
	require.NoError(t, err)
	assert.Equal(t, "running", revivedSession.Status)
	assert.NotEqual(t, session.PID, revivedSession.PID, "PID should change")

	// 5. Wait for second run to finish
	time.Sleep(500 * time.Millisecond)
	proc2, err := os.FindProcess(revivedSession.PID)
	if err == nil {
		proc2.Wait() // Reap zombie
	}

	// 6. Check logs
	logContent, err := sm.GetSessionLogContent(sessionName, 100)
	require.NoError(t, err)

	// Should contain "run1" twice (once from first run, once from second because we used the same command)
	// And the separator
	assert.Equal(t, 2, strings.Count(logContent, "run1"), "Log should contain 'run1' twice")
	assert.Contains(t, logContent, "Session Resumed/Revived", "Log should contain separator")
}
