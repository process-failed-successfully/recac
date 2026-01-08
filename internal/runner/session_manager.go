package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

// SessionState represents the state of a background session
type SessionState struct {
	Name          string    `json:"name"`
	PID           int       `json:"pid"`
	StartTime     time.Time `json:"start_time"`
	Command       []string `json:"command"`
	LogFile       string    `json:"log_file"`
	Workspace     string    `json:"workspace"`
	Status        string    `json:"status"`         // "running", "completed", "stopped", "error"
	Type          string    `json:"type"`           // "detached" or "interactive"
	AgentStateFile string   `json:"agent_state_file"` // Path to agent state file (.agent_state.json)
}

// SessionManager handles background session management
type SessionManager struct {
	sessionsDir string
}

// NewSessionManager creates a new session manager
func NewSessionManager() (*SessionManager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	sessionsDir := filepath.Join(homeDir, ".recac", "sessions")
	if err := os.MkdirAll(sessionsDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create sessions directory: %w", err)
	}

	return &SessionManager{
		sessionsDir: sessionsDir,
	}, nil
}

// NewSessionManagerWithDir creates a new session manager with a specific directory.
func NewSessionManagerWithDir(dir string) (*SessionManager, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create sessions directory: %w", err)
	}
	return &SessionManager{
		sessionsDir: dir,
	}, nil
}

// GetSessionPath returns the path to a session state file
func (sm *SessionManager) GetSessionPath(name string) string {
	return filepath.Join(sm.sessionsDir, name+".json")
}

// StartSession starts a session in detached mode
func (sm *SessionManager) StartSession(name string, command []string, workspace string) (*SessionState, error) {
	// Check if session already exists
	sessionPath := sm.GetSessionPath(name)
	if _, err := os.Stat(sessionPath); err == nil {
		// Session file exists, check if process is running
		existing, err := sm.LoadSession(name)
		if err == nil && sm.IsProcessRunning(existing.PID) {
			return nil, fmt.Errorf("session '%s' is already running (PID: %d)", name, existing.PID)
		}
		// Cleanup dead session file
		os.Remove(sessionPath)
	}

	// Create log file
	logFile := filepath.Join(sm.sessionsDir, name+".log")
	// Use OpenFile with O_EXCL to prevent TOCTOU race conditions and overwriting existing logs atomically
	// Use 0600 to restrict access to the owner only
	logFd, err := os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to create log file (safe): %w", err)
	}
	defer logFd.Close()

	// Create agent state file path in workspace (so it's accessible to the running process)
	agentStateFile := filepath.Join(workspace, ".agent_state.json")

	// Build command with proper arguments
	// Ensure executable path is absolute and exists
	execPath := command[0]
	if !filepath.IsAbs(execPath) {
		absPath, err := filepath.Abs(execPath)
		if err == nil {
			execPath = absPath
		}
	}
	
	// Resolve symlinks
	if resolved, err := filepath.EvalSymlinks(execPath); err == nil {
		execPath = resolved
	}
	
	// Verify executable exists and is accessible
	if stat, err := os.Stat(execPath); err != nil {
		return nil, fmt.Errorf("executable not found at %s: %w", execPath, err)
	} else if stat.Mode()&0111 == 0 {
		return nil, fmt.Errorf("executable %s is not executable", execPath)
	}
	
	cmd := exec.Command(execPath, command[1:]...)
	cmd.Stdout = logFd
	cmd.Stderr = logFd
	cmd.Dir = workspace
	cmd.Env = os.Environ() // Preserve environment

	// Start process in new session (detached from terminal)
	// Note: Setsid may not work in all environments (e.g., Docker containers without proper capabilities)
	// For now, we start without Setsid to ensure it works, even if not fully detached from terminal
	// The process will still run in background and output to log file
	// TODO: Add Setsid support when running in environments that support it
	// cmd.SysProcAttr = &syscall.SysProcAttr{
	// 	Setsid: true,
	// }

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start process: %w", err)
	}

	// Create session state
	session := &SessionState{
		Name:          name,
		PID:           cmd.Process.Pid,
		StartTime:     time.Now(),
		Command:       command,
		LogFile:       logFile,
		Workspace:     workspace,
		Status:        "running",
		Type:          "detached",
		AgentStateFile: agentStateFile,
	}

	// Save session state
	if err := sm.SaveSession(session); err != nil {
		// Try to kill the process if we can't save state
		cmd.Process.Kill()
		return nil, fmt.Errorf("failed to save session state: %w", err)
	}

	return session, nil
}

// LoadSession loads a session state from disk
func (sm *SessionManager) LoadSession(name string) (*SessionState, error) {
	sessionPath := sm.GetSessionPath(name)
	data, err := os.ReadFile(sessionPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	var session SessionState
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to parse session file: %w", err)
	}

	return &session, nil
}

// SaveSession saves a session state to disk
func (sm *SessionManager) SaveSession(session *SessionState) error {
	sessionPath := sm.GetSessionPath(session.Name)
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	if err := os.WriteFile(sessionPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write session file: %w", err)
	}

	return nil
}

// ListSessions returns all sessions
func (sm *SessionManager) ListSessions() ([]*SessionState, error) {
	entries, err := os.ReadDir(sm.sessionsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read sessions directory: %w", err)
	}

	var sessions []*SessionState
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		name := entry.Name()[:len(entry.Name())-5] // Remove .json extension
		session, err := sm.LoadSession(name)
		if err != nil {
			continue // Skip invalid session files
		}

		// Update status based on process state
		// Only update if status is "running" - preserve "stopped" and "error" statuses
		if session.Status == "running" && !sm.IsProcessRunning(session.PID) {
			session.Status = "completed"
			sm.SaveSession(session) // Update on disk
		}

		sessions = append(sessions, session)
	}

	return sessions, nil
}

// IsProcessRunning checks if a process is still running
func (sm *SessionManager) IsProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Signal 0 doesn't actually send a signal, but checks if process exists
	err = process.Signal(os.Signal(syscall.Signal(0)))
	return err == nil
}

// StopSessionFunc defines the type for the stop session function
var StopSessionFunc = (*SessionManager).StopSession

// StopSession stops a running session
func (sm *SessionManager) StopSession(name string) error {
	session, err := sm.LoadSession(name)
	if err != nil {
		return fmt.Errorf("session not found: %w", err)
	}

	if session.Status != "running" {
		return fmt.Errorf("session '%s' is not running (status: %s)", name, session.Status)
	}

	if !sm.IsProcessRunning(session.PID) {
		session.Status = "completed"
		sm.SaveSession(session)
		return fmt.Errorf("session '%s' is not running (process not found)", name)
	}

	// Send SIGTERM for graceful shutdown
	process, err := os.FindProcess(session.PID)
	if err != nil {
		return fmt.Errorf("failed to find process: %w", err)
	}

	if err := process.Signal(os.Interrupt); err != nil {
		return fmt.Errorf("failed to send signal: %w", err)
	}

	// Wait a bit for graceful shutdown
	time.Sleep(2 * time.Second)

	// If still running, force kill
	if sm.IsProcessRunning(session.PID) {
		if err := process.Kill(); err != nil {
			return fmt.Errorf("failed to kill process: %w", err)
		}
	}

	session.Status = "stopped"
	sm.SaveSession(session)

	return nil
}

// GetSessionLogs returns the log file path for a session
func (sm *SessionManager) GetSessionLogs(name string) (string, error) {
	session, err := sm.LoadSession(name)
	if err != nil {
		return "", fmt.Errorf("session not found: %w", err)
	}

	return session.LogFile, nil
}
