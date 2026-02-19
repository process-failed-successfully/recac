package runner

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionManager_StartSession_Errors(t *testing.T) {
	sm, cleanup := setupSessionManager(t)
	defer cleanup()

	// 1. Invalid Name
	_, err := sm.StartSession("../invalid", "goal", []string{"ls"}, sm.sessionsDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "path traversal characters detected")

	// 2. Executable not found
	_, err = sm.StartSession("exec-not-found", "goal", []string{"/non/existent/exec"}, sm.sessionsDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "executable not found")

	// 3. Executable not executable
	// Create a non-executable file
	nonExec := filepath.Join(sm.sessionsDir, "script.sh")
	os.WriteFile(nonExec, []byte("#!/bin/sh\necho hello"), 0644)
	_, err = sm.StartSession("exec-perm-error", "goal", []string{nonExec}, sm.sessionsDir)
	assert.Error(t, err)
	// Error message might vary by OS, but usually "permission denied" or "not executable"
	// The code checks mode & 0111
	assert.Contains(t, err.Error(), "is not executable")

	// 4. Session already running
	// Start a valid session first
	sleepCmd, err := exec.LookPath("sleep")
	if err == nil {
		_, err = sm.StartSession("running-session", "goal", []string{sleepCmd, "10"}, sm.sessionsDir)
		require.NoError(t, err)

		// Try to start again with same name
		_, err = sm.StartSession("running-session", "goal", []string{sleepCmd, "10"}, sm.sessionsDir)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "is already running")
	}
}

func TestSessionManager_StartSession_CleanupDead(t *testing.T) {
	sm, cleanup := setupSessionManager(t)
	defer cleanup()

	sleepCmd, err := exec.LookPath("sleep")
	if err != nil {
		t.Skip("sleep command not found")
	}

	// Create a "dead" session file manually
	sessionName := "dead-session"
	deadSession := &SessionState{
		Name: sessionName,
		PID:  999999, // Non-existent PID
		Status: "running",
	}
	err = sm.SaveSession(deadSession)
	require.NoError(t, err)

	// Start a new session with the same name - should succeed and cleanup
	_, err = sm.StartSession(sessionName, "goal", []string{sleepCmd, "1"}, sm.sessionsDir)
	assert.NoError(t, err)
}

func TestSessionManager_LoadSession_Errors(t *testing.T) {
	sm, cleanup := setupSessionManager(t)
	defer cleanup()

	// 1. Invalid Name
	_, err := sm.LoadSession("../invalid")
	assert.Error(t, err)

	// 2. File not found
	_, err = sm.LoadSession("non-existent")
	assert.Error(t, err)

	// 3. Corrupt JSON
	sessionName := "corrupt-session"
	os.WriteFile(sm.GetSessionPath(sessionName), []byte("{invalid-json"), 0600)
	_, err = sm.LoadSession(sessionName)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse session file")
}

func TestSessionManager_SaveSession_Errors(t *testing.T) {
	// Skip on Windows/Root as permissions are different
	if os.Geteuid() == 0 {
		t.Skip("Skipping permission test as root")
	}

	sm, cleanup := setupSessionManager(t)
	defer cleanup()

	// Make sessions dir read-only
	os.Chmod(sm.sessionsDir, 0500)
	defer os.Chmod(sm.sessionsDir, 0700)

	session := &SessionState{Name: "test-save-error"}
	err := sm.SaveSession(session)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to write session file")
}

func TestSessionManager_ArchiveSession_Errors(t *testing.T) {
	sm, cleanup := setupSessionManager(t)
	defer cleanup()

	// 1. Session not found
	err := sm.ArchiveSession("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session 'non-existent' not found")

	// 2. Rename error (simulate by locking file or permission)
	// Create session
	sessionName := "archive-error"
	session := &SessionState{Name: sessionName, Status: "completed", LogFile: filepath.Join(sm.sessionsDir, sessionName+".log")}
	sm.SaveSession(session)
	os.Create(session.LogFile)

	// Make archive dir read-only
	os.Chmod(sm.archivedSessionsDir, 0500)
	defer os.Chmod(sm.archivedSessionsDir, 0700)

	err = sm.ArchiveSession(sessionName)
	assert.Error(t, err)
	// Error message depends on OS, usually "permission denied"
}

func TestSessionManager_UnarchiveSession_Errors(t *testing.T) {
	sm, cleanup := setupSessionManager(t)
	defer cleanup()

	// 1. Archived session not found
	err := sm.UnarchiveSession("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "archived session 'non-existent' not found")

	// 2. Rename error
	sessionName := "unarchive-error"
	archivedSession := &SessionState{Name: sessionName}
	data, _ := json.Marshal(archivedSession)
	os.WriteFile(filepath.Join(sm.archivedSessionsDir, sessionName+".json"), data, 0600)
	os.Create(filepath.Join(sm.archivedSessionsDir, sessionName+".log"))

	// Make sessions dir read-only
	os.Chmod(sm.sessionsDir, 0500)
	defer os.Chmod(sm.sessionsDir, 0700)

	err = sm.UnarchiveSession(sessionName)
	assert.Error(t, err)
}

func TestSessionManager_RenameSession_Errors_More(t *testing.T) {
	sm, cleanup := setupSessionManager(t)
	defer cleanup()

	// Invalid names
	err := sm.RenameSession("../invalid", "valid")
	assert.Error(t, err)
	err = sm.RenameSession("valid", "../invalid")
	assert.Error(t, err)

	// Rename running session check covered in existing test, but verify specific error type if possible
	// The code returns fmt.Errorf("session '%s' is running ... %w", ..., ErrSessionRunning)
	// So we can check errors.Is(err, ErrSessionRunning)
}
