package runner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartSession_ErrorPaths(t *testing.T) {
	sm, cleanup := setupSessionManager(t)
	defer cleanup()

	// 1. Invalid session name
	_, err := sm.StartSession("", "goal", []string{"ls"}, sm.sessionsDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session name cannot be empty")

	_, err = sm.StartSession("../invalid", "goal", []string{"ls"}, sm.sessionsDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "path traversal characters detected")

	// 2. Executable not found
	_, err = sm.StartSession("exec-not-found", "goal", []string{"/non/existent/executable"}, sm.sessionsDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "executable not found")

	// 3. Executable not executable
	// Create a dummy non-executable file
	nonExecPath := filepath.Join(sm.sessionsDir, "dummy.txt")
	err = os.WriteFile(nonExecPath, []byte("dummy"), 0644)
	require.NoError(t, err)
	_, err = sm.StartSession("exec-perm", "goal", []string{nonExecPath}, sm.sessionsDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is not executable")

	// 4. Session already running
	// Create a fake running session file
	runningSessionName := "running-session"
	pid := os.Getpid()
	runningSession := &SessionState{
		Name:    runningSessionName,
		PID:     pid,
		Status:  "running",
		LogFile: filepath.Join(sm.sessionsDir, runningSessionName+".log"),
	}
	err = sm.SaveSession(runningSession)
	require.NoError(t, err)

	_, err = sm.StartSession(runningSessionName, "goal", []string{"ls"}, sm.sessionsDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is already running")
}

func TestLoadSession_ErrorPaths(t *testing.T) {
	sm, cleanup := setupSessionManager(t)
	defer cleanup()

	// 1. Invalid session name
	_, err := sm.LoadSession("")
	assert.Error(t, err)

	// 2. File not found
	_, err = sm.LoadSession("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read session file")

	// 3. Invalid JSON
	sessionName := "invalid-json"
	sessionPath := sm.GetSessionPath(sessionName)
	err = os.WriteFile(sessionPath, []byte("invalid json"), 0644)
	require.NoError(t, err)

	_, err = sm.LoadSession(sessionName)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse session file")
}

func TestStopSession_ErrorPaths(t *testing.T) {
	sm, cleanup := setupSessionManager(t)
	defer cleanup()

	// 1. Session not found
	err := sm.StopSession("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session not found")

	// 2. Session not running
	sessionName := "completed-session"
	session := &SessionState{
		Name:    sessionName,
		Status:  "completed",
		PID:     12345,
		LogFile: filepath.Join(sm.sessionsDir, sessionName+".log"),
	}
	err = sm.SaveSession(session)
	require.NoError(t, err)

	err = sm.StopSession(sessionName)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is not running")

	// 3. Process not found (already dead)
	sessionName = "dead-session"
	// Use a PID that is unlikely to exist (max PID on linux is usually 32768, but can be larger)
	// Or use a PID that definitely doesn't exist?
	// Finding a non-existent PID is tricky cross-platform without race.
	// But usually a large number works if not recycling fast.
	// Or kill a process then use its PID.
	// But IsProcessRunning uses Signal(0).

	// Create a dummy process then kill it.
	// But Wait, IsProcessRunning handles os.FindProcess returning process struct on unix even if dead?
	// Signal(0) checks if process exists.

	// Let's rely on IsProcessRunning returning false for a non-existent PID.
	// On Linux, os.FindProcess always succeeds.

	session = &SessionState{
		Name:    sessionName,
		Status:  "running",
		PID:     99999999, // Unlikely to exist
		LogFile: filepath.Join(sm.sessionsDir, sessionName+".log"),
	}
	err = sm.SaveSession(session)
	require.NoError(t, err)

	err = sm.StopSession(sessionName)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "process not found")

	// Verify status updated to completed
	loaded, err := sm.LoadSession(sessionName)
	require.NoError(t, err)
	assert.Equal(t, "completed", loaded.Status)
}

func TestRemoveSession_ErrorPaths(t *testing.T) {
	sm, cleanup := setupSessionManager(t)
	defer cleanup()

	// 1. Session not found
	err := sm.RemoveSession("non-existent", false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session 'non-existent' not found")

	// 2. Running session without force
	sessionName := "running-session"
	session := &SessionState{
		Name:    sessionName,
		Status:  "running",
		PID:     os.Getpid(),
		LogFile: filepath.Join(sm.sessionsDir, sessionName+".log"),
	}
	err = sm.SaveSession(session)
	require.NoError(t, err)

	err = sm.RemoveSession(sessionName, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is running")
}

func TestListSessions_Coverage(t *testing.T) {
	sm, cleanup := setupSessionManager(t)
	defer cleanup()

	// 1. Corrupted session file (should be skipped)
	err := os.WriteFile(filepath.Join(sm.sessionsDir, "corrupted.json"), []byte("bad json"), 0644)
	require.NoError(t, err)

	// 2. Valid session
	session := &SessionState{
		Name:    "valid",
		Status:  "completed",
		PID:     12345,
		LogFile: filepath.Join(sm.sessionsDir, "valid.log"),
	}
	err = sm.SaveSession(session)
	require.NoError(t, err)

	sessions, err := sm.ListSessions()
	require.NoError(t, err)

	// Should only find the valid one
	assert.Len(t, sessions, 1)
	assert.Equal(t, "valid", sessions[0].Name)
}

func TestListArchivedSessions_Coverage(t *testing.T) {
	sm, cleanup := setupSessionManager(t)
	defer cleanup()

	// 1. Corrupted archived session file (should be skipped)
	err := os.WriteFile(filepath.Join(sm.archivedSessionsDir, "corrupted.json"), []byte("bad json"), 0644)
	require.NoError(t, err)

	// 2. Valid archived session
	session := &SessionState{
		Name:    "archived",
		Status:  "completed",
		PID:     12345,
		LogFile: filepath.Join(sm.archivedSessionsDir, "archived.log"),
	}
	data, err := json.Marshal(session)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(sm.archivedSessionsDir, "archived.json"), data, 0644)
	require.NoError(t, err)

	sessions, err := sm.ListArchivedSessions()
	require.NoError(t, err)

	// Should only find the valid one
	assert.Len(t, sessions, 1)
	assert.Equal(t, "archived", sessions[0].Name)
}

func TestGetSessionLogContent_Coverage(t *testing.T) {
	sm, cleanup := setupSessionManager(t)
	defer cleanup()

	// 1. Session not found
	_, err := sm.GetSessionLogContent("non-existent", 10)
	assert.Error(t, err)

	// 2. Log file not found
	sessionName := "no-log"
	session := &SessionState{
		Name:    sessionName,
		Status:  "completed",
		LogFile: filepath.Join(sm.sessionsDir, "missing.log"),
	}
	err = sm.SaveSession(session)
	require.NoError(t, err)

	_, err = sm.GetSessionLogContent(sessionName, 10)
	assert.Error(t, err)

	// 3. Valid log file
	sessionName = "valid-log"
	logFile := filepath.Join(sm.sessionsDir, "valid.log")
	err = os.WriteFile(logFile, []byte("line1\nline2\nline3\n"), 0644)
	require.NoError(t, err)

	session = &SessionState{
		Name:    sessionName,
		Status:  "completed",
		LogFile: logFile,
	}
	err = sm.SaveSession(session)
	require.NoError(t, err)

	// Test lines <= 0 (all content)
	content, err := sm.GetSessionLogContent(sessionName, 0)
	require.NoError(t, err)
	assert.Contains(t, content, "line1\nline2\nline3")

	// Test lines > total lines
	content, err = sm.GetSessionLogContent(sessionName, 10)
	require.NoError(t, err)
	assert.Contains(t, content, "line1\nline2\nline3")

	// Test lines < total lines
	content, err = sm.GetSessionLogContent(sessionName, 2)
	require.NoError(t, err)
	assert.NotContains(t, content, "line1")
	assert.Contains(t, content, "line2\nline3")
}

func TestUpdateSessionStatus_Coverage(t *testing.T) {
	sm, cleanup := setupSessionManager(t)
	defer cleanup()

	// Create a "running" session but process is dead
	sessionName := "dead-running"
	session := &SessionState{
		Name:    sessionName,
		Status:  "running",
		PID:     99999999,
		LogFile: filepath.Join(sm.sessionsDir, sessionName+".log"),
		Workspace: sm.sessionsDir, // Use valid workspace for git SHA check attempt
	}
	err := sm.SaveSession(session)
	require.NoError(t, err)

	// ListSessions should detect it's dead and update status to completed
	sessions, err := sm.ListSessions()
	require.NoError(t, err)

	found := false
	for _, s := range sessions {
		if s.Name == sessionName {
			found = true
			assert.Equal(t, "completed", s.Status)
			assert.NotEmpty(t, s.EndTime)
			// EndCommitSHA might be empty if not a git repo, but that's handled
		}
	}
	assert.True(t, found)
}
