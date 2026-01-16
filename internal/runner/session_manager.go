package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"recac/internal/git"
	"strings"
	"syscall"
	"time"
)

// validateSessionName ensures the session name is safe to use in file paths
func validateSessionName(name string) error {
	if name == "" {
		return fmt.Errorf("session name cannot be empty")
	}
	if filepath.Base(name) != name {
		return fmt.Errorf("invalid session name '%s': path traversal characters detected", name)
	}
	return nil
}

// SessionState represents the state of a background session
type SessionState struct {
	Name           string    `json:"name"`
	PID            int       `json:"pid"`
	StartTime      time.Time `json:"start_time"`
	EndTime        time.Time `json:"end_time,omitempty"`
	Command        []string  `json:"command"`
	LogFile        string    `json:"log_file"`
	Workspace      string    `json:"workspace"`
	Status         string    `json:"status"` // "running", "paused", "completed", "stopped", "error"
	Type           string    `json:"type"`   // "detached" or "interactive"
	Goal           string    `json:"goal,omitempty"`
	Error          string    `json:"error,omitempty"`
	AgentStateFile string    `json:"agent_state_file"` // Path to agent state file (.agent_state.json)
	StartCommitSHA string    `json:"start_commit_sha,omitempty"`
	EndCommitSHA   string    `json:"end_commit_sha,omitempty"`
	ContainerID    string    `json:"container_id,omitempty"`
}

// SessionManager handles background session management
type SessionManager struct {
	sessionsDir         string
	archivedSessionsDir string
}

// ISessionManager defines the interface for session management.
type ISessionManager interface {
	ListSessions() ([]*SessionState, error)
	SaveSession(*SessionState) error
	LoadSession(name string) (*SessionState, error)
	StopSession(name string) error
	PauseSession(name string) error
	ResumeSession(name string) error
	GetSessionLogs(name string) (string, error)
	GetSessionLogContent(name string, lines int) (string, error)
	StartSession(name, goal string, command []string, workspace string) (*SessionState, error)
	GetSessionPath(name string) string
	IsProcessRunning(pid int) bool
	RemoveSession(name string, force bool) error
	RenameSession(oldName, newName string) error
	SessionsDir() string
	GetSessionGitDiffStat(name string) (string, error)
	ArchiveSession(name string) error
	UnarchiveSession(name string) error
	ListArchivedSessions() ([]*SessionState, error)
}

// NewSessionManager creates a new session manager
var NewSessionManager = func() (ISessionManager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	sessionsDir := filepath.Join(homeDir, ".recac", "sessions")
	archivedSessionsDir := filepath.Join(sessionsDir, "archived")

	if err := os.MkdirAll(sessionsDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create sessions directory: %w", err)
	}
	if err := os.MkdirAll(archivedSessionsDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create archived sessions directory: %w", err)
	}

	sm := &SessionManager{
		sessionsDir:         sessionsDir,
		archivedSessionsDir: archivedSessionsDir,
	}
	return sm, nil
}

// NewSessionManagerWithDir creates a new session manager with a specific directory.
func NewSessionManagerWithDir(dir string) (*SessionManager, error) {
	archivedSessionsDir := filepath.Join(dir, "archived")

	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create sessions directory: %w", err)
	}
	if err := os.MkdirAll(archivedSessionsDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create archived sessions directory: %w", err)
	}

	sm := &SessionManager{
		sessionsDir:         dir,
		archivedSessionsDir: archivedSessionsDir,
	}
	return sm, nil
}

// GetSessionPath returns the path to a session state file
func (sm *SessionManager) GetSessionPath(name string) string {
	return filepath.Join(sm.sessionsDir, name+".json")
}

// SessionsDir returns the root directory where sessions are stored.
func (sm *SessionManager) SessionsDir() string {
	return sm.sessionsDir
}

// StartSession starts a session in detached mode
func (sm *SessionManager) StartSession(name, goal string, command []string, workspace string) (*SessionState, error) {
	if err := validateSessionName(name); err != nil {
		return nil, err
	}

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
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start process: %w", err)
	}

	// Create session state
	session := &SessionState{
		Name:           name,
		PID:            cmd.Process.Pid,
		StartTime:      time.Now(),
		Command:        command,
		LogFile:        logFile,
		Workspace:      workspace,
		Status:         "running",
		Type:           "detached",
		Goal:           goal,
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
	if err := validateSessionName(name); err != nil {
		return nil, err
	}

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
	if err := validateSessionName(session.Name); err != nil {
		return err
	}

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

// PauseSession sends a SIGSTOP signal to pause a running session.
func (sm *SessionManager) PauseSession(name string) error {
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

	process, err := os.FindProcess(session.PID)
	if err != nil {
		return fmt.Errorf("failed to find process %d: %w", session.PID, err)
	}

	// On Unix-like systems, os.Signal is an interface, and syscall.Signal is a concrete type.
	// We send SIGSTOP to pause the process.
	if err := process.Signal(syscall.SIGSTOP); err != nil {
		return fmt.Errorf("failed to send SIGSTOP signal to process %d: %w", session.PID, err)
	}

	session.Status = "paused"
	return sm.SaveSession(session)
}

// ResumeSession sends a SIGCONT signal to resume a paused session.
func (sm *SessionManager) ResumeSession(name string) error {
	session, err := sm.LoadSession(name)
	if err != nil {
		return fmt.Errorf("session not found: %w", err)
	}

	if session.Status != "paused" {
		return fmt.Errorf("session '%s' is not paused (status: %s)", name, session.Status)
	}

	// A paused process is still "running" from the OS's perspective.
	if !sm.IsProcessRunning(session.PID) {
		session.Status = "stopped" // If it's not running anymore while paused, it's effectively stopped/crashed.
		sm.SaveSession(session)
		return fmt.Errorf("session '%s' is no longer running (process not found)", name)
	}

	process, err := os.FindProcess(session.PID)
	if err != nil {
		return fmt.Errorf("failed to find process %d: %w", session.PID, err)
	}

	// Send SIGCONT to resume the process.
	if err := process.Signal(syscall.SIGCONT); err != nil {
		return fmt.Errorf("failed to send SIGCONT signal to process %d: %w", session.PID, err)
	}

	session.Status = "running"
	return sm.SaveSession(session)
}

// ArchiveSession moves a session's state and log files to the archived directory.
func (sm *SessionManager) ArchiveSession(name string) error {
	session, err := sm.LoadSession(name)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("session '%s' not found", name)
		}
		return fmt.Errorf("could not load session '%s': %w", name, err)
	}

	if sm.IsProcessRunning(session.PID) {
		return fmt.Errorf("cannot archive running session '%s' (PID: %d)", name, session.PID)
	}

	// Move session state file (.json)
	oldSessionPath := sm.GetSessionPath(name)
	newSessionPath := filepath.Join(sm.archivedSessionsDir, filepath.Base(oldSessionPath))
	if err := os.Rename(oldSessionPath, newSessionPath); err != nil {
		return fmt.Errorf("failed to move session state file to archive: %w", err)
	}

	// Move log file (.log)
	oldLogPath := session.LogFile
	newLogPath := filepath.Join(sm.archivedSessionsDir, filepath.Base(oldLogPath))
	if err := os.Rename(oldLogPath, newLogPath); err != nil {
		// Rollback session file move
		os.Rename(newSessionPath, oldSessionPath)
		return fmt.Errorf("failed to move session log file to archive: %w", err)
	}

	return nil
}

// UnarchiveSession moves a session's state and log files from the archived directory back to the active sessions directory.
func (sm *SessionManager) UnarchiveSession(name string) error {
	// Note: We don't use LoadSession here because it looks in the active sessions directory.
	archivedSessionPath := filepath.Join(sm.archivedSessionsDir, name+".json")
	if _, err := os.Stat(archivedSessionPath); os.IsNotExist(err) {
		return fmt.Errorf("archived session '%s' not found", name)
	}

	activeSessionPath := sm.GetSessionPath(name)
	if _, err := os.Stat(activeSessionPath); err == nil {
		return fmt.Errorf("an active session named '%s' already exists", name)
	}

	// Move session state file (.json)
	if err := os.Rename(archivedSessionPath, activeSessionPath); err != nil {
		return fmt.Errorf("failed to move session state file from archive: %w", err)
	}

	// Move log file (.log)
	archivedLogPath := filepath.Join(sm.archivedSessionsDir, name+".log")
	activeLogPath := filepath.Join(sm.sessionsDir, name+".log")
	if err := os.Rename(archivedLogPath, activeLogPath); err != nil {
		// Rollback session file move
		os.Rename(activeSessionPath, archivedSessionPath)
		return fmt.Errorf("failed to move session log file from archive: %w", err)
	}

	return nil
}

// RenameSession renames a session, including its state and log files.
func (sm *SessionManager) RenameSession(oldName, newName string) error {
	if err := validateSessionName(oldName); err != nil {
		return err
	}
	if err := validateSessionName(newName); err != nil {
		return err
	}

	// 1. Load the session state for the old name.
	session, err := sm.LoadSession(oldName)
	if err != nil {
		// Re-check for NotExist because the error wrapping can be tricky.
		// This makes the check more robust for the "not found" case.
		sessionPath := sm.GetSessionPath(oldName)
		if _, statErr := os.Stat(sessionPath); os.IsNotExist(statErr) {
			return fmt.Errorf("session '%s' not found", oldName)
		}
		return fmt.Errorf("could not load session '%s': %w", oldName, err)
	}

	// 2. Prevent renaming of a running session.
	if sm.IsProcessRunning(session.PID) {
		return fmt.Errorf("session '%s' is running (PID: %d): %w", oldName, session.PID, ErrSessionRunning)
	}

	// 3. Check if a session with the new name already exists.
	newSessionPath := sm.GetSessionPath(newName)
	if _, err := os.Stat(newSessionPath); err == nil {
		return fmt.Errorf("a session named '%s' already exists", newName)
	}

	// 4. Rename the session state file (.json).
	oldSessionPath := sm.GetSessionPath(oldName)
	if err := os.Rename(oldSessionPath, newSessionPath); err != nil {
		return fmt.Errorf("failed to rename session state file: %w", err)
	}

	// 5. Rename the log file (.log).
	oldLogPath := session.LogFile
	newLogPath := filepath.Join(sm.sessionsDir, newName+".log")
	if err := os.Rename(oldLogPath, newLogPath); err != nil {
		// Attempt to roll back the session file rename.
		os.Rename(newSessionPath, oldSessionPath)
		return fmt.Errorf("failed to rename session log file: %w", err)
	}

	// 6. Update the session's internal state.
	session.Name = newName
	session.LogFile = newLogPath

	// 7. Save the updated session state to the newly named file.
	if err := sm.SaveSession(session); err != nil {
		// Attempt to roll back both renames.
		os.Rename(newSessionPath, oldSessionPath)
		os.Rename(newLogPath, oldLogPath)
		return fmt.Errorf("failed to save updated session state: %w", err)
	}

	return nil
}

// GetSessionGitDiffStat returns the `git diff --stat` output between a session's start and end commits.
func (sm *SessionManager) GetSessionGitDiffStat(name string) (string, error) {
	session, err := sm.LoadSession(name)
	if err != nil {
		return "", fmt.Errorf("could not load session '%s': %w", name, err)
	}

	if session.StartCommitSHA == "" || session.EndCommitSHA == "" || session.Workspace == "" {
		return "", nil // Not an error, just no diff to show
	}

	// Use the git client for consistency, though direct exec is also fine here.
	gitClient := git.NewClient()
	diff, err := gitClient.DiffStat(session.Workspace, session.StartCommitSHA, session.EndCommitSHA)
	if err != nil {
		return "", fmt.Errorf("failed to get git diff stat: %w", err)
	}

	return diff, nil
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
			session.EndTime = time.Now()

			// Get the ending commit SHA
			gitClient := git.NewClient()
			if session.Workspace != "" {
				sha, err := gitClient.CurrentCommitSHA(session.Workspace)
				if err == nil {
					session.EndCommitSHA = sha
				}
			}

			sm.SaveSession(session) // Update on disk
		}

		sessions = append(sessions, session)
	}

	return sessions, nil
}

// ListArchivedSessions returns all archived sessions
func (sm *SessionManager) ListArchivedSessions() ([]*SessionState, error) {
	entries, err := os.ReadDir(sm.archivedSessionsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read archived sessions directory: %w", err)
	}

	var sessions []*SessionState
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		sessionPath := filepath.Join(sm.archivedSessionsDir, entry.Name())
		data, err := os.ReadFile(sessionPath)
		if err != nil {
			continue // Skip invalid session files
		}

		var session SessionState
		if err := json.Unmarshal(data, &session); err != nil {
			continue // Skip corrupted session files
		}

		sessions = append(sessions, &session)
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
	session.EndTime = time.Now()

	// Get the ending commit SHA
	gitClient := git.NewClient()
	if session.Workspace != "" {
		sha, err := gitClient.CurrentCommitSHA(session.Workspace)
		if err == nil {
			session.EndCommitSHA = sha
		}
	}

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

// GetSessionLogContent returns the last N lines of the log file for a session.
func (sm *SessionManager) GetSessionLogContent(name string, lines int) (string, error) {
	session, err := sm.LoadSession(name)
	if err != nil {
		return "", fmt.Errorf("session not found: %w", err)
	}

	logData, err := os.ReadFile(session.LogFile)
	if err != nil {
		return "", fmt.Errorf("could not read log file %s: %w", session.LogFile, err)
	}

	logStr := string(logData)
	if lines <= 0 {
		return logStr, nil
	}

	logLines := strings.Split(strings.TrimSpace(logStr), "\n")
	if len(logLines) <= lines {
		return logStr, nil
	}

	start := len(logLines) - lines
	return strings.Join(logLines[start:], "\n"), nil
}

// ErrSessionRunning is returned when an operation cannot be performed on a running session without force.
var ErrSessionRunning = fmt.Errorf("session is running")

// RemoveSession deletes a session's state and log files from disk.
func (sm *SessionManager) RemoveSession(name string, force bool) error {
	session, err := sm.LoadSession(name)
	if err != nil {
		// Use os.IsNotExist to provide a cleaner "not found" message.
		if os.IsNotExist(err) {
			return fmt.Errorf("session '%s' not found", name)
		}
		return fmt.Errorf("could not load session '%s': %w", name, err)
	}

	// Check if the process is running and force flag is not provided
	if sm.IsProcessRunning(session.PID) && !force {
		return fmt.Errorf("session '%s' is running (PID: %d), use --force to remove: %w", name, session.PID, ErrSessionRunning)
	}

	// Remove session state file (.json)
	sessionPath := sm.GetSessionPath(name)
	err = os.Remove(sessionPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove session state file %s: %w", sessionPath, err)
	}

	// Remove log file (.log)
	logPath := session.LogFile
	err = os.Remove(logPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove session log file %s: %w", logPath, err)
	}

	return nil
}
