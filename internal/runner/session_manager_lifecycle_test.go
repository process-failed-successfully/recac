package runner

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionManager_StopSession_ForceKill(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping force kill test on Windows due to signal handling differences")
	}

	sm, cleanup := setupSessionManager(t)
	defer cleanup()

	// Create a script that ignores SIGTERM/SIGINT and sleeps
	// We use a shell script for this.
	scriptName := "ignore_sigterm.sh"
	scriptPath := filepath.Join(sm.SessionsDir(), scriptName)
	scriptContent := `#!/bin/sh
trap '' TERM INT
sleep 10
`
	err := os.WriteFile(scriptPath, []byte(scriptContent), 0755)
	require.NoError(t, err)

	// Start the session
	// We need to run it with sh
	shPath, err := exec.LookPath("sh")
	require.NoError(t, err)

	sessionName := "force-kill-test"
	cmd := []string{shPath, scriptPath}
	session, err := sm.StartSession(sessionName, "goal", cmd, sm.SessionsDir())
	require.NoError(t, err)

	// Verify it's running
	assert.Equal(t, "running", session.Status)
	assert.True(t, sm.IsProcessRunning(session.PID))

	// Stop session - this should trigger the force kill path after timeout
	// But the timeout is 2 seconds in StopSession.
	// To make test faster, we can't easily change the hardcoded timeout in StopSession.
	// So we have to wait > 2 seconds.
	start := time.Now()
	err = sm.StopSession(sessionName)
	duration := time.Since(start)

	require.NoError(t, err)
	assert.True(t, duration >= 2*time.Second, "StopSession should wait at least 2 seconds for graceful shutdown")

	// Verify it's stopped
	loaded, err := sm.LoadSession(sessionName)
	require.NoError(t, err)
	assert.Equal(t, "stopped", loaded.Status)

	// Reap zombie if any
	proc, err := os.FindProcess(session.PID)
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() {
		_, err := proc.Wait()
		done <- err
	}()

	select {
	case err := <-done:
		// On non-Unix systems or if not child, Wait might fail, but here we expect success or specific error.
		// Just logging error if strictly needed, but for now we care it returns.
		_ = err
	case <-time.After(5 * time.Second):
		t.Fatal("Timed out waiting for process to exit - kill failed?")
	}

	// Wait a tiny bit for OS to clean up
	time.Sleep(100 * time.Millisecond)
	assert.False(t, sm.IsProcessRunning(session.PID))
}

func TestSessionManager_ListSessions_UpdatesStatus(t *testing.T) {
	sm, cleanup := setupSessionManager(t)
	defer cleanup()

	// Create a session file that says "running" but points to a non-existent PID
	// We use a very large PID that is unlikely to exist.
	// However, on some systems PIDs wrap around.
	// Better approach: start a process, wait for it to die, then create the session file pointing to its PID.

	// Start a short lived process to get a valid PID that definitely exits
	cmd := exec.Command("echo", "done")
	err := cmd.Start()
	require.NoError(t, err)
	pid := cmd.Process.Pid
	cmd.Wait() // Wait for it to exit

	sessionName := "zombie-session"
	session := &SessionState{
		Name:      sessionName,
		PID:       pid,
		Status:    "running", // Lie about status
		StartTime: time.Now(),
		LogFile:   filepath.Join(sm.SessionsDir(), sessionName+".log"),
	}

	// Create dummy log file
	err = os.WriteFile(session.LogFile, []byte("log"), 0600)
	require.NoError(t, err)

	// Save the lying session
	data, err := json.Marshal(session)
	require.NoError(t, err)
	err = os.WriteFile(sm.GetSessionPath(sessionName), data, 0600)
	require.NoError(t, err)

	// List sessions - should detect process is gone and update status
	sessions, err := sm.ListSessions()
	require.NoError(t, err)

	require.Len(t, sessions, 1)
	assert.Equal(t, sessionName, sessions[0].Name)
	assert.Equal(t, "completed", sessions[0].Status)
	assert.False(t, sessions[0].EndTime.IsZero())

	// Verify it persisted the update
	loaded, err := sm.LoadSession(sessionName)
	require.NoError(t, err)
	assert.Equal(t, "completed", loaded.Status)
}
