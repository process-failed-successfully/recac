package e2e

import (
	"fmt"
	"recac/internal/runner"
)

type MockSessionManager struct {
	Sessions map[string]*runner.SessionState
}

func NewMockSessionManager() *MockSessionManager {
	return &MockSessionManager{
		Sessions: make(map[string]*runner.SessionState),
	}
}

func (m *MockSessionManager) ListSessions() ([]*runner.SessionState, error) {
	var sessions []*runner.SessionState
	for _, s := range m.Sessions {
		sessions = append(sessions, s)
	}
	return sessions, nil
}

func (m *MockSessionManager) SaveSession(s *runner.SessionState) error {
	m.Sessions[s.Name] = s
	return nil
}

func (m *MockSessionManager) LoadSession(name string) (*runner.SessionState, error) {
	s, ok := m.Sessions[name]
	if !ok {
		return nil, fmt.Errorf("session not found")
	}
	return s, nil
}

func (m *MockSessionManager) StopSession(name string) error {
	s, ok := m.Sessions[name]
	if !ok {
		return fmt.Errorf("session not found")
	}
	s.Status = "stopped"
	return nil
}

func (m *MockSessionManager) PauseSession(name string) error {
	s, ok := m.Sessions[name]
	if !ok {
		return fmt.Errorf("session not found")
	}
	s.Status = "paused"
	return nil
}

func (m *MockSessionManager) ResumeSession(name string) error {
	s, ok := m.Sessions[name]
	if !ok {
		return fmt.Errorf("session not found")
	}
	s.Status = "running"
	return nil
}

func (m *MockSessionManager) GetSessionLogs(name string) (string, error) {
	return "", nil
}

func (m *MockSessionManager) GetSessionLogContent(name string, lines int) (string, error) {
	return "", nil
}

func (m *MockSessionManager) StartSession(name, goal string, command []string, workspace string) (*runner.SessionState, error) {
	s := &runner.SessionState{
		Name:      name,
		Goal:      goal,
		Command:   command,
		Workspace: workspace,
		Status:    "running",
	}
	m.Sessions[name] = s
	return s, nil
}

func (m *MockSessionManager) GetSessionPath(name string) string {
	return ""
}

func (m *MockSessionManager) IsProcessRunning(pid int) bool {
	return true
}

func (m *MockSessionManager) RemoveSession(name string, force bool) error {
	delete(m.Sessions, name)
	return nil
}

func (m *MockSessionManager) RenameSession(oldName, newName string) error {
	s, ok := m.Sessions[oldName]
	if !ok {
		return fmt.Errorf("session not found")
	}
	delete(m.Sessions, oldName)
	s.Name = newName
	m.Sessions[newName] = s
	return nil
}

func (m *MockSessionManager) SessionsDir() string {
	return ""
}

func (m *MockSessionManager) GetSessionGitDiffStat(name string) (string, error) {
	return "", nil
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
