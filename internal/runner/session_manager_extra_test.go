package runner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSessionManager_GetSessionLogContent(t *testing.T) {
	sm, cleanup := setupSessionManager(t)
	defer cleanup()

	sessionName := "log-test-session"
	logContent := "line 1\nline 2\nline 3\nline 4\nline 5\n"

	// Create a dummy session and log file
	logPath := filepath.Join(sm.SessionsDir(), sessionName+".log")
	err := os.WriteFile(logPath, []byte(logContent), 0600)
	require.NoError(t, err)

	session := &SessionState{
		Name:    sessionName,
		LogFile: logPath,
		Status:  "completed",
	}
	err = sm.SaveSession(session)
	require.NoError(t, err)

	t.Run("returns full logs when lines is 0", func(t *testing.T) {
		content, err := sm.GetSessionLogContent(sessionName, 0)
		require.NoError(t, err)
		require.Equal(t, logContent, content)
	})

	t.Run("returns last N lines", func(t *testing.T) {
		content, err := sm.GetSessionLogContent(sessionName, 2)
		require.NoError(t, err)
		// Based on implementation: strings.Join(lines[start:], "\n")
		// "line 1...line 5" -> TrimSpace -> split -> ["line 1", ..., "line 5"]
		// Last 2: ["line 4", "line 5"] -> Join -> "line 4\nline 5"
		expectedTrimmed := "line 4\nline 5"
		require.Equal(t, expectedTrimmed, content)
	})

	t.Run("returns full logs when lines > total lines", func(t *testing.T) {
		content, err := sm.GetSessionLogContent(sessionName, 100)
		require.NoError(t, err)
		require.Equal(t, logContent, content)
	})

	t.Run("returns error for non-existent session", func(t *testing.T) {
		_, err := sm.GetSessionLogContent("non-existent", 10)
		require.Error(t, err)
		require.Contains(t, err.Error(), "session not found")
	})

	t.Run("returns error for non-existent log file", func(t *testing.T) {
		// Create session without log file
		badSessionName := "bad-session"
		badSession := &SessionState{
			Name:    badSessionName,
			LogFile: filepath.Join(sm.SessionsDir(), "missing.log"),
		}
		require.NoError(t, sm.SaveSession(badSession))

		_, err := sm.GetSessionLogContent(badSessionName, 10)
		require.Error(t, err)
		require.Contains(t, err.Error(), "could not read log file")
	})
}
