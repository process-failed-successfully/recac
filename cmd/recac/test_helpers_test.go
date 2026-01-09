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
	Sessions    map[string]*runner.SessionState
	FailOnLoad  bool
	FailOnList  bool
	ProcessDown bool
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
	return pid != 0 // Simple mock
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

func (m *MockSessionManager) SessionsDir() string {
	return "/tmp/recac/sessions"
}

func (m *MockSessionManager) SaveSession(session *runner.SessionState) error {
	m.Sessions[session.Name] = session
	return nil
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
