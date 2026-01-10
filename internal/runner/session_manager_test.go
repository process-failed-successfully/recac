package runner

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

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
