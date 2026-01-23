package runner

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionsDir(t *testing.T) {
	sm, cleanup := setupSessionManager(t)
	defer cleanup()

	expected := sm.sessionsDir
	assert.Equal(t, expected, sm.SessionsDir())
}

func TestGetSessionLogContent(t *testing.T) {
	sm, cleanup := setupSessionManager(t)
	defer cleanup()

	sessionName := "log-test-session"
	logContent := "line1\nline2\nline3\nline4\nline5"
	logFile := filepath.Join(sm.SessionsDir(), sessionName+".log")
	err := os.WriteFile(logFile, []byte(logContent), 0600)
	require.NoError(t, err)

	session := &SessionState{
		Name:    sessionName,
		LogFile: logFile,
	}
	err = sm.SaveSession(session)
	require.NoError(t, err)

	t.Run("Get all content", func(t *testing.T) {
		content, err := sm.GetSessionLogContent(sessionName, 0)
		assert.NoError(t, err)
		assert.Equal(t, logContent, content)
	})

	t.Run("Get last N lines", func(t *testing.T) {
		content, err := sm.GetSessionLogContent(sessionName, 2)
		assert.NoError(t, err)
		expected := "line4\nline5"
		assert.Equal(t, expected, content)
	})

	t.Run("Get more lines than available", func(t *testing.T) {
		content, err := sm.GetSessionLogContent(sessionName, 10)
		assert.NoError(t, err)
		assert.Equal(t, logContent, content)
	})

	t.Run("Session not found", func(t *testing.T) {
		_, err := sm.GetSessionLogContent("non-existent", 10)
		assert.Error(t, err)
	})

	t.Run("Log file not found", func(t *testing.T) {
		// Delete log file but keep session
		os.Remove(logFile)
		_, err := sm.GetSessionLogContent(sessionName, 10)
		assert.Error(t, err)
	})
}

func TestStartSession_Errors(t *testing.T) {
	sm, cleanup := setupSessionManager(t)
	defer cleanup()

	t.Run("Invalid Name", func(t *testing.T) {
		_, err := sm.StartSession("../badname", "goal", []string{"echo"}, ".")
		assert.Error(t, err)
	})

	t.Run("Executable Not Found", func(t *testing.T) {
		_, err := sm.StartSession("exec-fail", "goal", []string{"/non/existent/exec"}, ".")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "executable not found")
	})

	t.Run("Session Already Running", func(t *testing.T) {
		// Mock a running session by creating state file and pointing to our own PID
		pid := os.Getpid()
		sessionName := "already-running"
		session := &SessionState{
			Name: sessionName,
			PID:  pid,
		}
		sm.SaveSession(session)

		_, err := sm.StartSession(sessionName, "goal", []string{"echo"}, ".")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already running")
	})
}

func TestPauseResume_Errors(t *testing.T) {
	sm, cleanup := setupSessionManager(t)
	defer cleanup()

	t.Run("Pause Session Not Found", func(t *testing.T) {
		err := sm.PauseSession("missing")
		assert.Error(t, err)
	})

	t.Run("Resume Session Not Found", func(t *testing.T) {
		err := sm.ResumeSession("missing")
		assert.Error(t, err)
	})

	t.Run("Pause Phantom Session", func(t *testing.T) {
		// Create session with non-existent PID
		// PID 999999 is likely safe to assume not running (PID_MAX_LIMIT is usually lower on some systems, but safer to loop/find one)
		// Or just use a very large number.
		pid := 999999
		// Verify it's not running
		process, _ := os.FindProcess(pid)
		if err := process.Signal(syscall.Signal(0)); err == nil {
			t.Skip("PID 999999 exists, skipping phantom test")
		}

		name := "phantom-pause"
		session := &SessionState{
			Name:   name,
			PID:    pid,
			Status: "running",
		}
		sm.SaveSession(session)

		err := sm.PauseSession(name)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "process not found")

		// Verify status changed to completed
		updated, _ := sm.LoadSession(name)
		assert.Equal(t, "completed", updated.Status)
	})

	t.Run("Resume Phantom Session", func(t *testing.T) {
		pid := 999998
		process, _ := os.FindProcess(pid)
		if err := process.Signal(syscall.Signal(0)); err == nil {
			t.Skip("PID 999998 exists, skipping phantom test")
		}

		name := "phantom-resume"
		session := &SessionState{
			Name:   name,
			PID:    pid,
			Status: "paused",
		}
		sm.SaveSession(session)

		err := sm.ResumeSession(name)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "process not found")

		// Verify status changed to stopped
		updated, _ := sm.LoadSession(name)
		assert.Equal(t, "stopped", updated.Status)
	})
}
