package mocks

import (
	"fmt"
	"recac/internal/runner"
	"sort"
)

// MockSessionManager is a mock implementation of the ISessionManager interface for testing.
type MockSessionManager struct {
	Sessions map[string]*runner.SessionState
}

func (m *MockSessionManager) ListSessions() ([]*runner.SessionState, error) {
	var sessions []*runner.SessionState
	for _, s := range m.Sessions {
		sessions = append(sessions, s)
	}
	// Sort for consistent output
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].Name < sessions[j].Name
	})
	return sessions, nil
}

func (m *MockSessionManager) SaveSession(s *runner.SessionState) error {
	m.Sessions[s.Name] = s
	return nil
}

func (m *MockSessionManager) LoadSession(name string) (*runner.SessionState, error) {
	if s, ok := m.Sessions[name]; ok {
		return s, nil
	}
	return nil, fmt.Errorf("session not found")
}

func (m *MockSessionManager) StopSession(name string) error {
	if s, ok := m.Sessions[name]; ok {
		s.Status = "stopped"
		return nil
	}
	return fmt.Errorf("session not found")
}

func (m *MockSessionManager) GetSessionLogs(name string) (string, error) {
	return "mock logs", nil
}

func (m *MockSessionManager) StartSession(name string, command []string, workspace string) (*runner.SessionState, error) {
	return &runner.SessionState{Name: name, Status: "running"}, nil
}

func (m *MockSessionManager) GetSessionPath(name string) string {
	return fmt.Sprintf("/tmp/sessions/%s", name)
}

func (m *MockSessionManager) IsProcessRunning(pid int) bool {
	return true
}

func (m *MockSessionManager) RemoveSession(name string, force bool) error {
	delete(m.Sessions, name)
	return nil
}

func (m *MockSessionManager) RenameSession(oldName, newName string) error {
	if s, ok := m.Sessions[oldName]; ok {
		s.Name = newName
		m.Sessions[newName] = s
		delete(m.Sessions, oldName)
		return nil
	}
	return fmt.Errorf("session not found")
}

func (m *MockSessionManager) SessionsDir() string {
	return "/tmp/sessions"
}

func (m *MockSessionManager) GetSessionGitDiffStat(name string) (string, error) {
	return "mock diff stat", nil
}

func (m *MockSessionManager) ArchiveSession(name string) error {
	return nil
}

func (m *MockSessionManager) UnarchiveSession(name string) error {
	return nil
}

func (m *MockSessionManager) ListArchivedSessions() ([]*runner.SessionState, error) {
	return nil, nil
}
