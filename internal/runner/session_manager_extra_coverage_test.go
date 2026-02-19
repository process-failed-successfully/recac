package runner

import (
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSessionManager_StopSession_Kill(t *testing.T) {
	sm, cleanup := setupSessionManager(t)
	defer cleanup()

	// Start a process that ignores SIGINT/SIGTERM if possible.
	// "trap '' INT TERM; sleep 10" in shell.
	cmd := exec.Command("sh", "-c", "trap '' INT TERM; sleep 10")
	err := cmd.Start()
	if err != nil {
		t.Skip("Failed to start shell process")
	}

	// Ensure zombie is reaped
	go cmd.Wait()

	defer func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	}()

	pid := cmd.Process.Pid
	sessionName := "test-kill"
	session := &SessionState{
		Name:   sessionName,
		PID:    pid,
		Status: "running",
	}
	sm.SaveSession(session)

	// StopSession sends SIGINT, waits 2s, then Kills.
	start := time.Now()
	err = sm.StopSession(sessionName)
	assert.NoError(t, err)
	duration := time.Since(start)

	// Should take at least 2 seconds (wait time)
	assert.GreaterOrEqual(t, duration.Seconds(), 1.9)

	// Verify status
	updated, err := sm.LoadSession(sessionName)
	assert.NoError(t, err)
	assert.Equal(t, "stopped", updated.Status)

	// Process should be dead
	// Give the OS a moment to clean up the process entry
	dead := false
	for i := 0; i < 20; i++ {
		if !sm.IsProcessRunning(pid) {
			dead = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	assert.True(t, dead, "Process should be dead")
}
