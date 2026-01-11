package main

import (
	"bytes"
	"fmt"
	"strings"
	"time"
	"recac/internal/runner"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// MockSessionManager is a mock implementation of the ISessionManager interface.
type MockSessionManager struct {
	Sessions        map[string]*runner.SessionState
	FailOnLoad      bool
	FailOnList      bool
	ProcessDown     bool
	SessionsDirFunc func() string
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
func (m *MockSessionManager) StartSession(name string, command []string, workspace string) (*runner.SessionState, error) {
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
	if m.ProcessDown {
		return false
	}
	if pid == 0 {
		return false
	}
	// Find the session with this PID and check its status.
	for _, session := range m.Sessions {
		if session.PID == pid {
			return session.Status == "running"
		}
	}
	// Default to false for unknown PIDs
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
func (m *MockSessionManager) GetSessionLogs(name string) (string, error) {
	if session, ok := m.Sessions[name]; ok {
		return session.LogFile, nil
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

func (m *MockSessionManager) GetSessionGitDiffStat(name string) (string, error) {
	if session, ok := m.Sessions[name]; ok {
		if session.StartCommitSHA != "" && session.EndCommitSHA != "" {
			return " M README.md\n 1 file changed, 1 insertion(+)", nil
		}
		return "", nil // No diff if no commits
	}
	return "", fmt.Errorf("session not found")
}

// executeCommand executes a cobra command and returns its output.
func executeCommand(root *cobra.Command, args ...string) (string, error) {
	resetFlags(root)
	// Mock exit
	oldExit := exit
	exit = func(code int) {
		if code != 0 {
			panic(fmt.Sprintf("exit-%d", code))
		}
	}
	defer func() { exit = oldExit }()
	defer func() {
		if r := recover(); r != nil {
			if s, ok := r.(string); ok && strings.HasPrefix(s, "exit-") {
				// This is an expected exit, don't re-panic
				return
			}
			panic(r) // Re-panic actual panics
		}
	}()
	root.SetArgs(args)
	b := new(bytes.Buffer)
	root.SetOut(b)
	root.SetErr(b)
	// Mock Stdin to avoid hanging on interactive prompts (e.g. wizard)
	root.SetIn(bytes.NewBufferString(""))
	err := root.Execute()
	return b.String(), err
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
