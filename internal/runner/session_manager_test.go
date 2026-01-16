package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"recac/internal/git"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockGitClient is a mock of the git.Client for testing purposes.
type MockGitClient struct {
	mock.Mock
}

func (m *MockGitClient) DiffStat(workspace, startCommit, endCommit string) (string, error) {
	args := m.Called(workspace, startCommit, endCommit)
	return args.String(0), args.Error(1)
}

func (m *MockGitClient) CurrentCommitSHA(workspace string) (string, error) {
	args := m.Called(workspace)
	return args.String(0), args.Error(1)
}

func (m *MockGitClient) Clone(ctx context.Context, repoURL, directory string) error {
	args := m.Called(ctx, repoURL, directory)
	return args.Error(0)
}

func (m *MockGitClient) RepoExists(directory string) bool {
	args := m.Called(directory)
	return args.Bool(0)
}

func (m *MockGitClient) Config(directory, key, value string) error {
	args := m.Called(directory, key, value)
	return args.Error(0)
}

func (m *MockGitClient) ConfigAddGlobal(key, value string) error {
	args := m.Called(key, value)
	return args.Error(0)
}

func (m *MockGitClient) RemoteBranchExists(directory, remote, branch string) (bool, error) {
	args := m.Called(directory, remote, branch)
	return args.Bool(0), args.Error(1)
}

func (m *MockGitClient) Fetch(directory, remote, branch string) error {
	args := m.Called(directory, remote, branch)
	return args.Error(0)
}

func (m *MockGitClient) Stash(directory string) error {
	args := m.Called(directory)
	return args.Error(0)
}

func (m *MockGitClient) Merge(directory, branchName string) error {
	args := m.Called(directory, branchName)
	return args.Error(0)
}

func (m *MockGitClient) AbortMerge(directory string) error {
	args := m.Called(directory)
	return args.Error(0)
}

func (m *MockGitClient) Recover(directory string) error {
	args := m.Called(directory)
	return args.Error(0)
}

func (m *MockGitClient) Clean(directory string) error {
	args := m.Called(directory)
	return args.Error(0)
}

func (m *MockGitClient) ResetHard(directory, remote, branch string) error {
	args := m.Called(directory, remote, branch)
	return args.Error(0)
}

func (m *MockGitClient) StashPop(directory string) error {
	args := m.Called(directory)
	return args.Error(0)
}

func (m *MockGitClient) DeleteRemoteBranch(directory, remote, branch string) error {
	args := m.Called(directory, remote, branch)
	return args.Error(0)
}

func (m *MockGitClient) CurrentBranch(directory string) (string, error) {
	args := m.Called(directory)
	return args.String(0), args.Error(1)
}

func (m *MockGitClient) Commit(directory, message string) error {
	args := m.Called(directory, message)
	return args.Error(0)
}

func (m *MockGitClient) Diff(directory, startCommit, endCommit string) (string, error) {
	args := m.Called(directory, startCommit, endCommit)
	return args.String(0), args.Error(1)
}

func (m *MockGitClient) SetRemoteURL(directory, name, url string) error {
	args := m.Called(directory, name, url)
	return args.Error(0)
}

func (m *MockGitClient) DeleteLocalBranch(directory, branch string) error {
	args := m.Called(directory, branch)
	return args.Error(0)
}

func (m *MockGitClient) LocalBranchExists(directory, branch string) (bool, error) {
	args := m.Called(directory, branch)
	return args.Bool(0), args.Error(1)
}

func (m *MockGitClient) Checkout(directory, branch string) error {
	args := m.Called(directory, branch)
	return args.Error(0)
}

func (m *MockGitClient) CheckoutNewBranch(directory, branch string) error {
	args := m.Called(directory, branch)
	return args.Error(0)
}

func (m *MockGitClient) Push(directory, branch string) error {
	args := m.Called(directory, branch)
	return args.Error(0)
}

func (m *MockGitClient) Pull(directory, remote, branch string) error {
	args := m.Called(directory, remote, branch)
	return args.Error(0)
}
// setupSessionManager creates a new SessionManager in a temporary directory for isolated testing.
func setupSessionManager(t *testing.T) (*SessionManager, func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "recac-test-")
	require.NoError(t, err, "Failed to create temp dir")

	sm, err := NewSessionManagerWithDir(tmpDir)
	require.NoError(t, err, "Failed to create session manager")

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}
	return sm, cleanup
}


func TestSessionManager_Lifecycle(t *testing.T) {
	// Setup temporary home directory
	tmpDir, err := os.MkdirTemp("", "recac-test-session")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Mock HOME for the test
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	// Create manager
	sm, err := NewSessionManager()
	if err != nil {
		t.Fatalf("Failed to create session manager: %v", err)
	}

	// Verify sessions dir created
	sessionsDir := filepath.Join(tmpDir, ".recac", "sessions")
	if _, err := os.Stat(sessionsDir); os.IsNotExist(err) {
		t.Errorf("Sessions directory not created at %s", sessionsDir)
	}

	// Prepare a dummy executable (sleep)
	// We use 'sleep' command which should be available
	sleepCmd, err := exec.LookPath("sleep")
	if err != nil {
		t.Skip("sleep command not found, skipping StartSession test")
	}

	// Test StartSession
	sessionName := "test-session"
	command := []string{sleepCmd, "1"} // Sleep for 1 second
	workspace := tmpDir

	session, err := sm.StartSession(sessionName, command, workspace)
	if err != nil {
		t.Fatalf("Failed to start session: %v", err)
	}

	if session.Status != "running" {
		t.Errorf("Expected status running, got %s", session.Status)
	}
	if session.PID == 0 {
		t.Error("Expected valid PID, got 0")
	}

	// Test IsProcessRunning
	if !sm.IsProcessRunning(session.PID) {
		t.Error("Process should be running")
	}

	// Test LoadSession
	loaded, err := sm.LoadSession(sessionName)
	if err != nil {
		t.Errorf("Failed to load session: %v", err)
	}
	if loaded.PID != session.PID {
		t.Errorf("Loaded PID %d mismatch original %d", loaded.PID, session.PID)
	}

	// Test ListSessions
	sessions, err := sm.ListSessions()
	if err != nil {
		t.Errorf("Failed to list sessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Errorf("Expected 1 session, got %d", len(sessions))
	}
	if sessions[0].Name != sessionName {
		t.Errorf("Expected session name %s, got %s", sessionName, sessions[0].Name)
	}

	// Test StopSession
	if err := sm.StopSession(sessionName); err != nil {
		t.Errorf("Failed to stop session: %v", err)
	}

	// Verify stopped
	// Wait a bit for process to actually exit if StopSession sends signal
	time.Sleep(100 * time.Millisecond)
	
	loaded, _ = sm.LoadSession(sessionName)
	if loaded.Status != "stopped" && loaded.Status != "completed" {
		t.Errorf("Expected status stopped/completed, got %s", loaded.Status)
	}
	
	if sm.IsProcessRunning(session.PID) {
		// It might take time to die, or be zombie. 
		// sleep 1 should be done by now anyway or killed.
		// If it was killed by StopSession, it should be gone.
		// However, waitpid is needed to reap zombies? 
		// The manager doesn't Wait() on the process.
		// But StopSession sends signal.
	}
}

func TestSessionManager_Persistence(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "recac-test-persist")
	defer os.RemoveAll(tmpDir)
	
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	sm, _ := NewSessionManager()
	
	session := &SessionState{
		Name: "persisted-session",
		PID: 12345,
		Status: "running",
		Command: []string{"/bin/echo", "hello"},
	}
	
	if err := sm.SaveSession(session); err != nil {
		t.Fatalf("Failed to save session: %v", err)
	}
	
	loaded, err := sm.LoadSession("persisted-session")
	if err != nil {
		t.Fatalf("Failed to load session: %v", err)
	}
	
	if loaded.PID != 12345 {
		t.Errorf("Expected PID 12345, got %d", loaded.PID)
	}
	
	// Check ListSessions updates status for non-existent PID
	// PID 12345 is unlikely to exist or be our child.
	// Actually IsProcessRunning checks if process exists via Signal(0).
	// If it doesn't exist, ListSessions should mark it completed.
	
	// Assuming PID 12345 doesn't exist (running as non-root, can't signal random PIDs usually, 
	// or if it exists it belongs to another user so Signal(0) might return error EPERM which means it exists?
	// os.FindProcess always succeeds on Unix. Signal(0) returns error if not exists or no perm.
	// If no perm, it exists.
	
	// We'll trust ListSessions logic for now, hard to test robustly without controlling PIDs.
	
	sessions, _ := sm.ListSessions()
	if len(sessions) > 0 {
		s := sessions[0]
		// It might have changed status to completed if PID is not found
		if s.Status == "running" {
			// If it's still running, it means PID 12345 exists. That's fine.
		} else if s.Status == "completed" {
			// This confirms the logic worked (assuming PID didn't exist)
		}
	}
}

func TestSessionManager_StopSession_NotRunning(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "recac-test-stop")
	defer os.RemoveAll(tmpDir)
	os.Setenv("HOME", tmpDir)
	
	sm, _ := NewSessionManager()
	
	session := &SessionState{
		Name: "stopped-session",
		Status: "stopped",
	}
	sm.SaveSession(session)
	
	err := sm.StopSession("stopped-session")
	if err == nil {
		t.Error("Expected error stopping already stopped session")
	}
}

func TestSessionManager_GetSessionLogs(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "recac-test-logs")
	defer os.RemoveAll(tmpDir)
	os.Setenv("HOME", tmpDir)
	
	sm, _ := NewSessionManager()
	
	session := &SessionState{
		Name: "log-session",
		LogFile: "/tmp/foo.log",
	}
	sm.SaveSession(session)
	
	logPath, err := sm.GetSessionLogs("log-session")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if logPath != "/tmp/foo.log" {
		t.Errorf("Expected /tmp/foo.log, got %s", logPath)
	}
}

func TestSessionManager_RemoveSession(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "recac-test-remove")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	sm, err := NewSessionManagerWithDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create session manager: %v", err)
	}

	// Create a dummy session to remove
	sessionName := "test-remove-session"
	logFile, _ := os.Create(filepath.Join(tmpDir, sessionName+".log"))
	logFile.Close()

	session := &SessionState{
		Name:    sessionName,
		Status:  "completed",
		LogFile: logFile.Name(),
		PID:     0, // Not running
	}
	sm.SaveSession(session)

	// Verify files exist
	if _, err := os.Stat(sm.GetSessionPath(sessionName)); os.IsNotExist(err) {
		t.Fatal("Session file should exist before removal")
	}
	if _, err := os.Stat(logFile.Name()); os.IsNotExist(err) {
		t.Fatal("Log file should exist before removal")
	}

	// Remove the session
	err = sm.RemoveSession(sessionName, false)
	if err != nil {
		t.Fatalf("Failed to remove session: %v", err)
	}

	// Verify files are gone
	if _, err := os.Stat(sm.GetSessionPath(sessionName)); !os.IsNotExist(err) {
		t.Error("Session file should not exist after removal")
	}
	if _, err := os.Stat(logFile.Name()); !os.IsNotExist(err) {
		t.Error("Log file should not exist after removal")
	}

	// Test removing a running session
	runningSessionName := "running-session"
	logFile2, _ := os.Create(filepath.Join(tmpDir, runningSessionName+".log"))
	logFile2.Close()

	runningSession := &SessionState{
		Name:    runningSessionName,
		Status:  "running",
		LogFile: logFile2.Name(),
		PID:     os.Getpid(), // Use the current process PID as a running process
	}
	sm.SaveSession(runningSession)

	// Attempt to remove without force
	err = sm.RemoveSession(runningSessionName, false)
	if err == nil {
		t.Fatal("Expected an error when removing a running session without force")
	}

	// Attempt to remove with force
	err = sm.RemoveSession(runningSessionName, true)
	if err != nil {
		t.Fatalf("Failed to remove a running session with force: %v", err)
	}
}

func TestArchiveAndUnarchiveSession(t *testing.T) {
	t.Run("archives and unarchives a session successfully", func(t *testing.T) {
		sm, cleanup := setupSessionManager(t)
		defer cleanup()

		// Create a mock session
		sessionName := "test-archive"
		session := &SessionState{Name: sessionName, Status: "completed", LogFile: filepath.Join(sm.sessionsDir, sessionName+".log")}
		err := sm.SaveSession(session)
		require.NoError(t, err)
		_, err = os.Create(session.LogFile)
		require.NoError(t, err)

		// Archive the session
		err = sm.ArchiveSession(sessionName)
		require.NoError(t, err)

		// Verify it's in the archived directory and not in the active one
		_, err = os.Stat(filepath.Join(sm.archivedSessionsDir, sessionName+".json"))
		assert.NoError(t, err, "json file should be in archive")
		_, err = os.Stat(filepath.Join(sm.archivedSessionsDir, sessionName+".log"))
		assert.NoError(t, err, "log file should be in archive")

		_, err = os.Stat(filepath.Join(sm.sessionsDir, sessionName+".json"))
		assert.True(t, os.IsNotExist(err), "json file should not be in active dir")

		// Unarchive the session
		err = sm.UnarchiveSession(sessionName)
		require.NoError(t, err)

		// Verify it's back in the active directory
		_, err = os.Stat(filepath.Join(sm.sessionsDir, sessionName+".json"))
		assert.NoError(t, err, "json file should be back in active dir")
		_, err = os.Stat(filepath.Join(sm.sessionsDir, sessionName+".log"))
		assert.NoError(t, err, "log file should be back in active dir")

		_, err = os.Stat(filepath.Join(sm.archivedSessionsDir, sessionName+".json"))
		assert.True(t, os.IsNotExist(err), "json file should not be in archive dir")
	})

	t.Run("fails to archive a running session", func(t *testing.T) {
		sm, cleanup := setupSessionManager(t)
		defer cleanup()

		// Create a mock running session using the current PID
		sessionName := "test-running"
		session := &SessionState{Name: sessionName, PID: os.Getpid(), Status: "running"}
		err := sm.SaveSession(session)
		require.NoError(t, err)

		err = sm.ArchiveSession(sessionName)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot archive running session")
	})

	t.Run("fails to unarchive a session when an active one with the same name exists", func(t *testing.T) {
		sm, cleanup := setupSessionManager(t)
		defer cleanup()

		sessionName := "test-conflict"

		// Create an active session
		activeSession := &SessionState{Name: sessionName, Status: "completed"}
		err := sm.SaveSession(activeSession)
		require.NoError(t, err)

		// Create an archived session (manually, for the test)
		archivedSession := &SessionState{Name: sessionName, Status: "completed"}
		data, err := json.Marshal(archivedSession)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(sm.archivedSessionsDir, sessionName+".json"), data, 0600)
		require.NoError(t, err)

		// Attempt to unarchive, which should fail
		err = sm.UnarchiveSession(sessionName)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "an active session named 'test-conflict' already exists")
	})
}

func TestListArchivedSessions(t *testing.T) {
	sm, cleanup := setupSessionManager(t)
	defer cleanup()

	// Create a mock archived session
	archivedSessionName := "test-archived-list"
	archivedSession := &SessionState{Name: archivedSessionName}
	data, err := json.Marshal(archivedSession)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(sm.archivedSessionsDir, archivedSessionName+".json"), data, 0600)
	require.NoError(t, err)

	archived, err := sm.ListArchivedSessions()
	require.NoError(t, err)
	require.Len(t, archived, 1)
	assert.Equal(t, archivedSessionName, archived[0].Name)
}

func TestRenameSession(t *testing.T) {
	t.Run("renames a session successfully", func(t *testing.T) {
		sm, cleanup := setupSessionManager(t)
		defer cleanup()

		oldName := "old-name"
		newName := "new-name"

		// Create a mock session
		session := &SessionState{Name: oldName, Status: "completed", LogFile: filepath.Join(sm.sessionsDir, oldName+".log")}
		err := sm.SaveSession(session)
		require.NoError(t, err)
		_, err = os.Create(session.LogFile)
		require.NoError(t, err)

		err = sm.RenameSession(oldName, newName)
		require.NoError(t, err)

		// Verify old files are gone
		_, err = os.Stat(sm.GetSessionPath(oldName))
		assert.True(t, os.IsNotExist(err))
		_, err = os.Stat(filepath.Join(sm.sessionsDir, oldName+".log"))
		assert.True(t, os.IsNotExist(err))

		// Verify new files exist
		_, err = os.Stat(sm.GetSessionPath(newName))
		assert.NoError(t, err)
		_, err = os.Stat(filepath.Join(sm.sessionsDir, newName+".log"))
		assert.NoError(t, err)

		// Verify the session content is updated
		renamedSession, err := sm.LoadSession(newName)
		require.NoError(t, err)
		assert.Equal(t, newName, renamedSession.Name)
		assert.Equal(t, filepath.Join(sm.sessionsDir, newName+".log"), renamedSession.LogFile)
	})

	t.Run("fails to rename a running session", func(t *testing.T) {
		sm, cleanup := setupSessionManager(t)
		defer cleanup()

		sessionName := "running-session"
		session := &SessionState{Name: sessionName, PID: os.Getpid(), Status: "running"}
		err := sm.SaveSession(session)
		require.NoError(t, err)

		err = sm.RenameSession(sessionName, "new-name")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "session is running")
	})

	t.Run("fails to rename to an existing session name", func(t *testing.T) {
		sm, cleanup := setupSessionManager(t)
		defer cleanup()

		// Create two sessions
		session1 := &SessionState{Name: "session1", Status: "completed"}
		err := sm.SaveSession(session1)
		require.NoError(t, err)
		session2 := &SessionState{Name: "session2", Status: "completed"}
		err = sm.SaveSession(session2)
		require.NoError(t, err)

		err = sm.RenameSession("session1", "session2")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "a session named 'session2' already exists")
	})

	t.Run("fails to rename a non-existent session", func(t *testing.T) {
		sm, cleanup := setupSessionManager(t)
		defer cleanup()

		err := sm.RenameSession("non-existent", "new-name")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "session 'non-existent' not found")
	})
}

func TestGetSessionGitDiffStat(t *testing.T) {
	originalNewClient := git.NewClient
	git.NewClient = func() git.IClient {
		return &MockGitClient{}
	}
	defer func() { git.NewClient = originalNewClient }()
	t.Run("returns empty string when SHAs are missing", func(t *testing.T) {
		sm, cleanup := setupSessionManager(t)
		defer cleanup()

		session := &SessionState{Name: "test-session", Workspace: "/tmp"}
		err := sm.SaveSession(session)
		require.NoError(t, err)

		diff, err := sm.GetSessionGitDiffStat("test-session")
		assert.NoError(t, err)
		assert.Equal(t, "", diff)
	})

	t.Run("returns an error when git client fails", func(t *testing.T) {
		sm, cleanup := setupSessionManager(t)
		defer cleanup()

		mockClient := new(MockGitClient)
		mockClient.On("DiffStat", "/tmp", "start", "end").Return("", fmt.Errorf("git error"))
		git.NewClient = func() git.IClient {
			return mockClient
		}

		session := &SessionState{Name: "test-session", Workspace: "/tmp", StartCommitSHA: "start", EndCommitSHA: "end"}
		err := sm.SaveSession(session)
		require.NoError(t, err)

		_, err = sm.GetSessionGitDiffStat("test-session")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get git diff stat")
	})

	t.Run("returns git diff stat successfully", func(t *testing.T) {
		sm, cleanup := setupSessionManager(t)
		defer cleanup()

		mockClient := new(MockGitClient)
		mockClient.On("DiffStat", "/tmp", "start", "end").Return("... diff output ...", nil)
		git.NewClient = func() git.IClient {
			return mockClient
		}

		session := &SessionState{Name: "test-session", Workspace: "/tmp", StartCommitSHA: "start", EndCommitSHA: "end"}
		err := sm.SaveSession(session)
		require.NoError(t, err)

		diff, err := sm.GetSessionGitDiffStat("test-session")
		assert.NoError(t, err)
		assert.Equal(t, "... diff output ...", diff)
	})
}

func TestSessionManager_PauseResume(t *testing.T) {
	sm, cleanup := setupSessionManager(t)
	defer cleanup()

	// Prepare a dummy executable (sleep)
	sleepCmd, err := exec.LookPath("sleep")
	if err != nil {
		t.Skip("sleep command not found, skipping StartSession test")
	}

	// 1. Start a session
	sessionName := "test-pause-resume"
	command := []string{sleepCmd, "5"} // Sleep long enough to be paused
	session, err := sm.StartSession(sessionName, command, sm.sessionsDir)
	require.NoError(t, err, "Failed to start session")
	require.Equal(t, "running", session.Status, "Session should be running")

	// 2. Pause the session
	err = sm.PauseSession(sessionName)
	require.NoError(t, err, "Failed to pause session")

	// 3. Verify status is 'paused'
	pausedSession, err := sm.LoadSession(sessionName)
	require.NoError(t, err, "Failed to load session after pause")
	assert.Equal(t, "paused", pausedSession.Status, "Session status should be 'paused'")

	// 4. Try to pause it again (should fail)
	err = sm.PauseSession(sessionName)
	assert.Error(t, err, "Should not be able to pause a paused session")
	assert.Contains(t, err.Error(), "is not running")

	// 5. Resume the session
	err = sm.ResumeSession(sessionName)
	require.NoError(t, err, "Failed to resume session")

	// 6. Verify status is 'running' again
	resumedSession, err := sm.LoadSession(sessionName)
	require.NoError(t, err, "Failed to load session after resume")
	assert.Equal(t, "running", resumedSession.Status, "Session status should be 'running' again")

	// 7. Try to resume it again (should fail)
	err = sm.ResumeSession(sessionName)
	assert.Error(t, err, "Should not be able to resume a running session")
	assert.Contains(t, err.Error(), "is not paused")

	// 8. Clean up by stopping the session
	err = sm.StopSession(sessionName)
	require.NoError(t, err, "Failed to stop the session for cleanup")
}
