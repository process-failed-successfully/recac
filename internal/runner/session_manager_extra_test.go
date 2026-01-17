package runner

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetSessionLogContent(t *testing.T) {
	tmpDir := t.TempDir()
	sm, err := NewSessionManagerWithDir(tmpDir)
	assert.NoError(t, err)

	sessionName := "test-session"
	logContent := "line1\nline2\nline3\nline4\nline5"
	logFile := filepath.Join(tmpDir, sessionName+".log")

	err = os.WriteFile(logFile, []byte(logContent), 0600)
	assert.NoError(t, err)

	session := &SessionState{
		Name:      sessionName,
		Status:    "stopped",
		StartTime: time.Now(),
		LogFile:   logFile,
	}
	err = sm.SaveSession(session)
	assert.NoError(t, err)

	t.Run("Get all content (lines <= 0)", func(t *testing.T) {
		content, err := sm.GetSessionLogContent(sessionName, 0)
		assert.NoError(t, err)
		assert.Equal(t, logContent, content)
	})

	t.Run("Get last 2 lines", func(t *testing.T) {
		content, err := sm.GetSessionLogContent(sessionName, 2)
		assert.NoError(t, err)
		assert.Equal(t, "line4\nline5", content)
	})

	t.Run("Get more lines than available", func(t *testing.T) {
		content, err := sm.GetSessionLogContent(sessionName, 10)
		assert.NoError(t, err)
		assert.Equal(t, logContent, content)
	})

	t.Run("Session not found", func(t *testing.T) {
		_, err := sm.GetSessionLogContent("non-existent", 10)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "session not found")
	})

	t.Run("Log file not found", func(t *testing.T) {
		// Create session without log file
		s2 := &SessionState{
			Name:    "no-log-session",
			LogFile: filepath.Join(tmpDir, "missing.log"),
		}
		sm.SaveSession(s2)

		_, err := sm.GetSessionLogContent("no-log-session", 10)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "could not read log file")
	})
}
