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

// setupSessionManagerTest helper to reduce boilerplate
func setupSessionManagerTest(t *testing.T) (*SessionManager, string) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "recac-test-coverage-")
	require.NoError(t, err)

	sm, err := NewSessionManagerWithDir(tmpDir)
	require.NoError(t, err)

	return sm, tmpDir
}

func TestStopSession_ForceKill(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping signal test on Windows")
	}

	sm, tmpDir := setupSessionManagerTest(t)
	defer os.RemoveAll(tmpDir)

	// Create a script that ignores SIGTERM
	scriptPath := filepath.Join(tmpDir, "ignore_term.sh")
	scriptContent := `#!/bin/sh
trap "echo 'Ignored SIGTERM'" TERM
sleep 10
`
	err := os.WriteFile(scriptPath, []byte(scriptContent), 0755)
	require.NoError(t, err)

	// Start the session
	cmd := exec.Command(scriptPath)
	logFile, err := os.Create(filepath.Join(tmpDir, "session.log"))
	require.NoError(t, err)
	defer logFile.Close()
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	err = cmd.Start()
	require.NoError(t, err)

	session := &SessionState{
		Name:      "stubborn-session",
		PID:       cmd.Process.Pid,
		Status:    "running",
		LogFile:   logFile.Name(),
		StartTime: time.Now(),
	}
	err = sm.SaveSession(session)
	require.NoError(t, err)

	// Stop the session. This should attempt graceful shutdown, fail, wait, and then force kill.
	// We expect it to take at least 2 seconds (the hardcoded sleep).
	start := time.Now()
	err = sm.StopSession("stubborn-session")
	duration := time.Since(start)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, duration.Seconds(), 2.0, "Should have waited for graceful shutdown")

	// Since we started the process with cmd.Start(), we are the parent.
	// When StopSession kills it, it becomes a zombie until we Wait() for it.
	// IsProcessRunning returns true for zombies.
	// We must reap the process to make it truly disappear from the OS table.
	go func() {
		cmd.Wait()
	}()

	// Give a small moment for Wait to reap
	time.Sleep(100 * time.Millisecond)

	// Verify process is gone
	assert.False(t, sm.IsProcessRunning(session.PID), "Process should be killed")

	// Verify session status is updated
	loaded, err := sm.LoadSession("stubborn-session")
	require.NoError(t, err)
	assert.Equal(t, "stopped", loaded.Status)
}

func TestArchiveSession_Failures(t *testing.T) {
	sm, tmpDir := setupSessionManagerTest(t)
	defer os.RemoveAll(tmpDir)

	// 1. Session Running
	sessionName := "running-archive"
	session := &SessionState{
		Name:   sessionName,
		Status: "running",
		PID:    os.Getpid(), // Self PID is always running
	}
	err := sm.SaveSession(session)
	require.NoError(t, err)

	err = sm.ArchiveSession(sessionName)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot archive running session")

	// 2. File Move Fail (Simulated by missing source file)
	// Create a session entry but delete the actual file before archiving (though LoadSession checks existence)
	// Better: Create session, but make destination directory read-only?
	// Note: Rename works on directory permissions.
	// If we make archivedSessionsDir read-only, it should fail.

	if runtime.GOOS != "windows" {
		sessionName = "fail-move"
		session = &SessionState{
			Name:    sessionName,
			Status:  "completed",
			LogFile: filepath.Join(sm.sessionsDir, sessionName+".log"),
		}
		sm.SaveSession(session)
		os.Create(session.LogFile)

		// Make archived dir read-only
		err = os.Chmod(sm.archivedSessionsDir, 0500)
		require.NoError(t, err)
		defer os.Chmod(sm.archivedSessionsDir, 0700)

		err = sm.ArchiveSession(sessionName)
		assert.Error(t, err)
		// Error message depends on OS, but usually contains "permission denied"
		// or "failed to move session state file"
	}
}

func TestUnarchiveSession_Failures(t *testing.T) {
	sm, tmpDir := setupSessionManagerTest(t)
	defer os.RemoveAll(tmpDir)

	// 1. Active session exists
	name := "conflict"
	// Create in archive
	archivedPath := filepath.Join(sm.archivedSessionsDir, name+".json")
	os.WriteFile(archivedPath, []byte("{}"), 0600)
	// Create in active
	activePath := filepath.Join(sm.sessionsDir, name+".json")
	os.WriteFile(activePath, []byte("{}"), 0600)

	err := sm.UnarchiveSession(name)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "active session named 'conflict' already exists")

	// 2. Archive not found
	err = sm.UnarchiveSession("missing")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "archived session 'missing' not found")
}

func TestSaveSession_Failures(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping chmod test on Windows")
	}

	sm, tmpDir := setupSessionManagerTest(t)
	defer os.RemoveAll(tmpDir)

	// Make sessions dir read-only
	err := os.Chmod(sm.sessionsDir, 0500)
	require.NoError(t, err)
	defer os.Chmod(sm.sessionsDir, 0700)

	session := &SessionState{Name: "fail-save"}
	err = sm.SaveSession(session)
	assert.Error(t, err)
	// Error typically "permission denied" on creating file
}

func TestListArchivedSessions_Corrupt(t *testing.T) {
	sm, tmpDir := setupSessionManagerTest(t)
	defer os.RemoveAll(tmpDir)

	// Create valid session
	valid := &SessionState{Name: "valid"}
	data, _ := json.Marshal(valid)
	os.WriteFile(filepath.Join(sm.archivedSessionsDir, "valid.json"), data, 0600)

	// Create corrupt session
	os.WriteFile(filepath.Join(sm.archivedSessionsDir, "corrupt.json"), []byte("{invalid-json"), 0600)

	// Create non-json file
	os.WriteFile(filepath.Join(sm.archivedSessionsDir, "ignored.txt"), []byte("data"), 0600)

	sessions, err := sm.ListArchivedSessions()
	require.NoError(t, err)

	// Should only find the valid one
	assert.Len(t, sessions, 1)
	assert.Equal(t, "valid", sessions[0].Name)
}

func TestRemoveSession_Running(t *testing.T) {
	sm, tmpDir := setupSessionManagerTest(t)
	defer os.RemoveAll(tmpDir)

	sessionName := "running-remove"
	session := &SessionState{
		Name:   sessionName,
		Status: "running",
		PID:    os.Getpid(),
	}
	sm.SaveSession(session)

	// Fail without force
	err := sm.RemoveSession(sessionName, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is running")

	// Success with force
	err = sm.RemoveSession(sessionName, true)
	assert.NoError(t, err)

	_, err = os.Stat(sm.GetSessionPath(sessionName))
	assert.True(t, os.IsNotExist(err))
}
