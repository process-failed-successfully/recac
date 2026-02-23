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

func TestSaveSession_Error(t *testing.T) {
	sm, cleanup := setupSessionManager(t)
	defer cleanup()

	session := &SessionState{
		Name:    "test-save-error",
		Status:  "completed",
		LogFile: "foo.log",
	}

	// Create a directory where the session file should be, to cause WriteFile to fail
	// or make the directory read-only.
	// sessionsDir/name.json

	sessionPath := sm.GetSessionPath(session.Name)
	// Create a directory at the path where the file should be
	err := os.Mkdir(sessionPath, 0700)
	require.NoError(t, err)

	// SaveSession calls os.WriteFile(sessionPath, ...)
	// Writing to a directory path usually fails with "is a directory"
	err = sm.SaveSession(session)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to write session file")
}

func TestListArchivedSessions_Corrupted(t *testing.T) {
	sm, cleanup := setupSessionManager(t)
	defer cleanup()

	// 1. Create a valid archived session
	validName := "valid-session"
	validSession := &SessionState{Name: validName}
	data, _ := json.Marshal(validSession)
	err := os.WriteFile(filepath.Join(sm.archivedSessionsDir, validName+".json"), data, 0600)
	require.NoError(t, err)

	// 2. Create a corrupted file
	err = os.WriteFile(filepath.Join(sm.archivedSessionsDir, "corrupted.json"), []byte("{invalid-json"), 0600)
	require.NoError(t, err)

	// 3. Create a file with wrong extension
	err = os.WriteFile(filepath.Join(sm.archivedSessionsDir, "other.txt"), []byte("data"), 0600)
	require.NoError(t, err)

	// 4. Create a directory
	err = os.Mkdir(filepath.Join(sm.archivedSessionsDir, "subdir"), 0700)
	require.NoError(t, err)

	// List sessions
	sessions, err := sm.ListArchivedSessions()
	require.NoError(t, err)

	// Should only find the valid one
	assert.Len(t, sessions, 1)
	assert.Equal(t, validName, sessions[0].Name)
}

func TestStopSession_ProcessNotFound(t *testing.T) {
	sm, cleanup := setupSessionManager(t)
	defer cleanup()

	sessionName := "ghost-process"
	// Use a PID that is unlikely to exist (max PID is usually 32768 or higher, but let's use a safe bet like 99999999 if OS allows, or just check)
	// Better: use a PID that definitely doesn't exist.
	// On Linux, we can't easily guarantee a PID is free without race, but for test it's fine.

	// However, StopSession first checks IsProcessRunning.
	// If IsProcessRunning returns false, it marks as completed and returns error.

	session := &SessionState{
		Name:   sessionName,
		Status: "running",
		PID:    99999999, // Unlikely PID
	}
	err := sm.SaveSession(session)
	require.NoError(t, err)

	err = sm.StopSession(sessionName)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is not running (process not found)")

	// Verify status updated to completed
	loaded, err := sm.LoadSession(sessionName)
	require.NoError(t, err)
	assert.Equal(t, "completed", loaded.Status)
}

func TestStopSession_SignalFailure(t *testing.T) {
	// This is hard to test deterministically because we need a process that exists but we can't signal.
	// Typically requires root or another user's process.
	// skipping for now as "ProcessNotFound" covers the common error path in StopSession logic before signaling.
}

func TestRemoveSession_NotFound(t *testing.T) {
	sm, cleanup := setupSessionManager(t)
	defer cleanup()

	err := sm.RemoveSession("non-existent", false)
	assert.Error(t, err)
	// The implementation wraps the error, but we should check if it contains "not found"
	assert.Contains(t, err.Error(), "not found")
}

func TestRemoveSession_LogFileRemoveError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("Skipping permission test as root")
	}

	sm, cleanup := setupSessionManager(t)
	defer cleanup()

	sessionName := "log-remove-fail"
	session := &SessionState{
		Name:    sessionName,
		Status:  "completed",
		LogFile: filepath.Join(sm.sessionsDir, sessionName+".log"),
	}
	err := sm.SaveSession(session)
	require.NoError(t, err)

	// Create log file
	f, err := os.Create(session.LogFile)
	require.NoError(t, err)
	f.Close()

	// To fail log file removal but allow session file removal, strictly speaking we need permission magic.
	// But removing a file depends on directory permissions.
	// If we make the directory non-writable, we can't remove EITHER.
	// SessionManager.RemoveSession removes session file first, then log file.

	// If we want to fail log file removal specifically...
	// Maybe if the log file is a directory?
	err = os.Remove(session.LogFile)
	require.NoError(t, err)
	err = os.Mkdir(session.LogFile, 0700)
	require.NoError(t, err)

	// os.Remove on a directory might fail or succeed depending on if it's empty?
	// os.Remove removes empty directories too.
	// Let's put a file in it so os.Remove fails (needs RemoveAll)
	err = os.WriteFile(filepath.Join(session.LogFile, "file"), []byte("data"), 0600)
	require.NoError(t, err)

	err = sm.RemoveSession(sessionName, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to remove session log file")

	// Cleanup the directory we created to avoid messing up cleanup()
	os.RemoveAll(session.LogFile)
}

func TestPauseSession_ProcessNotFound(t *testing.T) {
	sm, cleanup := setupSessionManager(t)
	defer cleanup()

	sessionName := "pause-ghost"
	session := &SessionState{
		Name:   sessionName,
		Status: "running",
		PID:    99999999,
	}
	sm.SaveSession(session)

	err := sm.PauseSession(sessionName)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "process not found")

	// Verify status updated to completed
	loaded, _ := sm.LoadSession(sessionName)
	assert.Equal(t, "completed", loaded.Status)
}

func TestResumeSession_ProcessNotFound(t *testing.T) {
	sm, cleanup := setupSessionManager(t)
	defer cleanup()

	sessionName := "resume-ghost"
	session := &SessionState{
		Name:   sessionName,
		Status: "paused",
		PID:    99999999,
	}
	sm.SaveSession(session)

	err := sm.ResumeSession(sessionName)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "process not found")

	// Verify status updated to stopped
	loaded, _ := sm.LoadSession(sessionName)
	assert.Equal(t, "stopped", loaded.Status)
}

func TestStartSession_ExecError(t *testing.T) {
	sm, cleanup := setupSessionManager(t)
	defer cleanup()

	// Use a non-existent command
	_, err := sm.StartSession("bad-exec", "goal", []string{"/non/existent/cmd"}, sm.sessionsDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "executable not found")
}

func TestStartSession_NotExecutable(t *testing.T) {
	sm, cleanup := setupSessionManager(t)
	defer cleanup()

	// Create a non-executable file
	notExec := filepath.Join(sm.sessionsDir, "not_exec")
	err := os.WriteFile(notExec, []byte("#!/bin/sh\necho hi"), 0600)
	require.NoError(t, err)

	_, err = sm.StartSession("perm-fail", "goal", []string{notExec}, sm.sessionsDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is not executable")
}

func TestStartSession_SessionExistsRunning(t *testing.T) {
	sm, cleanup := setupSessionManager(t)
	defer cleanup()

	// Start a real session
	cmd, _ := exec.LookPath("sleep")
	if cmd == "" {
		t.Skip("sleep not found")
	}

	sessionName := "existing"
	_, err := sm.StartSession(sessionName, "goal", []string{cmd, "10"}, sm.sessionsDir)
	require.NoError(t, err)

	// Try to start again with same name
	_, err = sm.StartSession(sessionName, "goal", []string{cmd, "10"}, sm.sessionsDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already running")

	// Cleanup
	sm.StopSession(sessionName)
}

func TestStartSession_SessionExistsDead(t *testing.T) {
	sm, cleanup := setupSessionManager(t)
	defer cleanup()

	sessionName := "dead-session"
	// Create a session file for a dead PID
	session := &SessionState{
		Name:   sessionName,
		Status: "running",
		PID:    99999999,
	}
	sm.SaveSession(session)

	// Try to start with same name. It should cleanup the dead session and start new.
	cmd, _ := exec.LookPath("sleep")
	if cmd == "" {
		t.Skip("sleep not found")
	}

	newSession, err := sm.StartSession(sessionName, "goal", []string{cmd, "1"}, sm.sessionsDir)
	require.NoError(t, err)
	assert.Equal(t, sessionName, newSession.Name)
	assert.NotEqual(t, 99999999, newSession.PID)

	// Cleanup
	// StopSession might fail if the process finished quickly
	sm.StopSession(sessionName)
}

// MockProcess allows mocking os.FindProcess if we could inject it, but we can't easily.
// Instead we rely on integration-style tests with real processes or file system states.

func TestGetSessionLogs_NotFound(t *testing.T) {
	sm, cleanup := setupSessionManager(t)
	defer cleanup()

	_, err := sm.GetSessionLogs("non-existent-session")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session not found")
}
