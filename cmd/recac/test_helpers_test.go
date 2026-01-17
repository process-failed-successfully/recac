package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"recac/internal/runner"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
)

// setupTestSessionManager creates a real SessionManager in a temporary directory for integration tests.
func setupTestSessionManager(t *testing.T) (*runner.SessionManager, func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "recac-cmd-test-")
	require.NoError(t, err, "Failed to create temp dir")

	sm, err := runner.NewSessionManagerWithDir(tmpDir)
	require.NoError(t, err, "Failed to create session manager")

	// Override the factory to use our real, temp session manager.
	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return sm, nil
	}

	cleanup := func() {
		os.RemoveAll(tmpDir)
		// Restore the original factory after the test.
		sessionManagerFactory = originalFactory
	}

	return sm, cleanup
}

// MockSessionManager is a mock implementation of the ISessionManager interface.
type MockSessionManager struct {
	Sessions                  map[string]*runner.SessionState
	FailOnLoad                bool
	FailOnList                bool
	IsProcessRunningFunc      func(pid int) bool
	SessionsDirFunc           func() string
	GetSessionGitDiffStatFunc func(name string) (string, error)
}

func (m *MockSessionManager) SessionsDir() string {
	if m.SessionsDirFunc != nil {
		return m.SessionsDirFunc()
	}
	return "/tmp/recac/sessions" // Default mock path
}

func NewMockSessionManager() *MockSessionManager {
	return &MockSessionManager{
		Sessions: make(map[string]*runner.SessionState),
	}
}
func (m *MockSessionManager) StartSession(name, goal string, command []string, workspace string) (*runner.SessionState, error) {
	if _, exists := m.Sessions[name]; exists {
		return nil, fmt.Errorf("session '%s' already exists", name)
	}
	session := &runner.SessionState{
		Name:      name,
		PID:       99999, // Mock PID
		StartTime: time.Now(),
		Status:    "running",
		Command:   command,
		Workspace: workspace,
		LogFile:   "/tmp/mock.log",
	}
	m.Sessions[name] = session
	return session, nil
}
func (m *MockSessionManager) LoadSession(name string) (*runner.SessionState, error) {
	if m.FailOnLoad {
		return nil, fmt.Errorf("failed to load session")
	}
	if session, ok := m.Sessions[name]; ok {
		return session, nil
	}
	return nil, fmt.Errorf("session not found")
}

func (m *MockSessionManager) ListSessions() ([]*runner.SessionState, error) {
	if m.FailOnList {
		return nil, fmt.Errorf("failed to list sessions")
	}
	var sessions []*runner.SessionState
	for _, s := range m.Sessions {
		sessions = append(sessions, s)
	}
	return sessions, nil
}

func (m *MockSessionManager) IsProcessRunning(pid int) bool {
	if m.IsProcessRunningFunc != nil {
		return m.IsProcessRunningFunc(pid)
	}
	if pid == 0 {
		return false
	}
	// Default behavior: check if any session with this PID is 'running'
	for _, s := range m.Sessions {
		if s.PID == pid && s.Status == "running" {
			return true
		}
	}
	return false
}

func (m *MockSessionManager) StopSession(name string) error {
	if session, ok := m.Sessions[name]; ok {
		session.Status = "stopped"
		session.EndTime = time.Now()
		return nil
	}
	return fmt.Errorf("session not found")
}

func (m *MockSessionManager) PauseSession(name string) error {
	if session, ok := m.Sessions[name]; ok {
		if session.Status != "running" {
			return fmt.Errorf("session is not running")
		}
		session.Status = "paused"
		return nil
	}
	return fmt.Errorf("session not found")
}

func (m *MockSessionManager) ResumeSession(name string) error {
	if session, ok := m.Sessions[name]; ok {
		if session.Status != "paused" {
			return fmt.Errorf("session is not paused")
		}
		session.Status = "running"
		return nil
	}
	return fmt.Errorf("session not found")
}

func (m *MockSessionManager) GetSessionLogs(name string) (string, error) {
	if session, ok := m.Sessions[name]; ok {
		return session.LogFile, nil
	}
	return "", fmt.Errorf("session not found")
}

func (m *MockSessionManager) GetSessionLogContent(name string, lines int) (string, error) {
	if session, ok := m.Sessions[name]; ok {
		// In a real scenario, we'd read the file. Here, we'll just return a mock string.
		// We can make this more sophisticated if needed (e.g., storing mock logs).
		if session.LogFile != "" {
			mockLogs := "line 1\nline 2\nline 3\nline 4\nline 5\n"
			if lines > 0 {
				logLines := strings.Split(strings.TrimSpace(mockLogs), "\n")
				if len(logLines) > lines {
					return strings.Join(logLines[len(logLines)-lines:], "\n"), nil
				}
				return mockLogs, nil
			}
			return mockLogs, nil
		}
		return "", nil // No log file specified
	}
	return "", fmt.Errorf("session not found")
}

func (m *MockSessionManager) GetSessionPath(name string) string {
	return fmt.Sprintf("/tmp/recac/sessions/%s.json", name)
}

func (m *MockSessionManager) SaveSession(session *runner.SessionState) error {
	m.Sessions[session.Name] = session
	return nil
}

func (m *MockSessionManager) RemoveSession(name string, force bool) error {
	session, ok := m.Sessions[name]
	if !ok {
		return fmt.Errorf("session not found")
	}

	if m.IsProcessRunning(session.PID) && !force {
		// Wrap the specific error so errors.Is() works
		return fmt.Errorf("session is running, use --force to remove: %w", runner.ErrSessionRunning)
	}

	delete(m.Sessions, name)
	return nil
}

func (m *MockSessionManager) RenameSession(oldName, newName string) error {
	session, ok := m.Sessions[oldName]
	if !ok {
		return fmt.Errorf("session not found")
	}

	if m.IsProcessRunning(session.PID) {
		return runner.ErrSessionRunning
	}

	if _, exists := m.Sessions[newName]; exists {
		return fmt.Errorf("session '%s' already exists", newName)
	}

	delete(m.Sessions, oldName)
	session.Name = newName
	m.Sessions[newName] = session

	return nil
}

func (m *MockSessionManager) GetSessionGitDiffStat(name string) (string, error) {
	if m.GetSessionGitDiffStatFunc != nil {
		return m.GetSessionGitDiffStatFunc(name)
	}
	if session, ok := m.Sessions[name]; ok {
		if session.StartCommitSHA != "" && session.EndCommitSHA != "" {
			return " M README.md\n 1 file changed, 1 insertion(+)", nil
		}
		return "", nil // No diff if no commits
	}
	return "", fmt.Errorf("session not found")
}

func (m *MockSessionManager) ArchiveSession(name string) error {
	// This is a simplified mock. A real implementation would move files.
	if session, ok := m.Sessions[name]; ok {
		if m.IsProcessRunning(session.PID) {
			return fmt.Errorf("cannot archive running session")
		}
		session.Status = "archived" // Use status to simulate archival for the mock
		return nil
	}
	return fmt.Errorf("session not found")
}

func (m *MockSessionManager) UnarchiveSession(name string) error {
	if session, ok := m.Sessions[name]; ok {
		if session.Status == "archived" {
			session.Status = "completed" // Restore to a non-running state
			return nil
		}
		return fmt.Errorf("session is not archived")
	}
	return fmt.Errorf("session not found")
}

func (m *MockSessionManager) ListArchivedSessions() ([]*runner.SessionState, error) {
	var archived []*runner.SessionState
	for _, s := range m.Sessions {
		if s.Status == "archived" {
			archived = append(archived, s)
		}
	}
	return archived, nil
}

// executeCommand executes a cobra command and returns its output.
func executeCommand(root *cobra.Command, args ...string) (output string, err error) {
	resetFlags(root)
	b := new(bytes.Buffer)

	// Mock exit
	oldExit := exit
	exit = func(code int) {
		if code != 0 {
			panic(fmt.Sprintf("exit-%d", code))
		}
	}
	defer func() { exit = oldExit }()

	// Use a defer with recover to handle our mocked exit(1)
	defer func() {
		if r := recover(); r != nil {
			if s, ok := r.(string); ok && strings.HasPrefix(s, "exit-") {
				// This is an expected exit. We capture the buffer content
				// and return it, suppressing the panic.
				output = b.String()
				err = nil // An exit is not a Go error
				return
			}
			// This was a real panic, so re-panic
			panic(r)
		}
	}()

	root.SetArgs(args)
	root.SetOut(b)
	root.SetErr(b)
	// Mock Stdin to avoid hanging on interactive prompts (e.g. wizard)
	root.SetIn(bytes.NewBufferString(""))

	err = root.Execute()
	output = b.String() // Capture output on the non-panic path
	return
}

// resetFlags resets all flags to their default values.
func resetFlags(cmd *cobra.Command) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Changed {
			f.Value.Set(f.DefValue)
			f.Changed = false
		}
	})
	for _, c := range cmd.Commands() {
		resetFlags(c)
	}
}
func newRootCmd() (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(outBuf)
	rootCmd.SetErr(errBuf)
	return rootCmd, outBuf, errBuf
}

// MockGitClient is a mock implementation of the IGitClient interface.
type MockGitClient struct {
	CheckoutFunc         func(repoPath, commitOrBranch string) error
	DiffFunc             func(repoPath, commitA, commitB string) (string, error)
	DiffStatFunc         func(repoPath, commitA, commitB string) (string, error)
	CurrentCommitSHAFunc func(repoPath string) (string, error)
	LogFunc              func(repoPath string, args ...string) (string, error)
	ResetHardFunc        func(directory, remote, branch string) error
	ResetFunc            func(directory, target string) error
}

func (m *MockGitClient) Checkout(repoPath, commitOrBranch string) error {
	if m.CheckoutFunc != nil {
		return m.CheckoutFunc(repoPath, commitOrBranch)
	}
	return nil
}

func (m *MockGitClient) Diff(repoPath, commitA, commitB string) (string, error) {
	if m.DiffFunc != nil {
		return m.DiffFunc(repoPath, commitA, commitB)
	}
	return "mock diff", nil
}

func (m *MockGitClient) DiffStat(repoPath, commitA, commitB string) (string, error) {
	if m.DiffStatFunc != nil {
		return m.DiffStatFunc(repoPath, commitA, commitB)
	}
	return "mock diff --stat", nil
}

func (m *MockGitClient) CurrentCommitSHA(repoPath string) (string, error) {
	if m.CurrentCommitSHAFunc != nil {
		return m.CurrentCommitSHAFunc(repoPath)
	}
	return "mock-sha", nil
}

func (m *MockGitClient) Log(repoPath string, args ...string) (string, error) {
	if m.LogFunc != nil {
		return m.LogFunc(repoPath, args...)
	}
	return "mock log", nil
}

func (m *MockGitClient) ResetHard(directory, remote, branch string) error {
	if m.ResetHardFunc != nil {
		return m.ResetHardFunc(directory, remote, branch)
	}
	return nil
}

func (m *MockGitClient) Reset(directory, target string) error {
	if m.ResetFunc != nil {
		return m.ResetFunc(directory, target)
	}
	return nil
}

// Stubs to satisfy interface
func (m *MockGitClient) Clone(ctx context.Context, repoURL, directory string) error { return nil }
func (m *MockGitClient) RepoExists(directory string) bool                           { return true }
func (m *MockGitClient) Config(directory, key, value string) error              { return nil }
func (m *MockGitClient) ConfigAddGlobal(key, value string) error                { return nil }
func (m *MockGitClient) RemoteBranchExists(directory, remote, branch string) (bool, error) {
	return true, nil
}
func (m *MockGitClient) Fetch(directory, remote, branch string) error     { return nil }
func (m *MockGitClient) CheckoutNewBranch(directory, branch string) error { return nil }
func (m *MockGitClient) Push(directory, branch string) error              { return nil }
func (m *MockGitClient) Pull(directory, remote, branch string) error      { return nil }
func (m *MockGitClient) Stash(directory string) error                     { return nil }
func (m *MockGitClient) Merge(directory, branchName string) error         { return nil }
func (m *MockGitClient) AbortMerge(directory string) error                { return nil }
func (m *MockGitClient) Recover(directory string) error                   { return nil }
func (m *MockGitClient) Clean(directory string) error                     { return nil }
func (m *MockGitClient) StashPop(directory string) error                  { return nil }
func (m *MockGitClient) DeleteRemoteBranch(directory, remote, branch string) error {
	return nil
}
func (m *MockGitClient) CurrentBranch(directory string) (string, error) { return "master", nil }
func (m *MockGitClient) Commit(directory, message string) error         { return nil }
func (m *MockGitClient) SetRemoteURL(directory, name, url string) error { return nil }
func (m *MockGitClient) DeleteLocalBranch(directory, branch string) error {
	return nil
}
func (m *MockGitClient) LocalBranchExists(directory, branch string) (bool, error) {
	return true, nil
}
