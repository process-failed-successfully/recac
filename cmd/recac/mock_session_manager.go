
package main

import (
	"recac/internal/runner"
	"time"
)

// MockSessionManager is a mock implementation of the ISessionManager interface for testing.
type MockSessionManager struct {
	Sessions    []*runner.SessionState
	ListErr     error
	SaveErr     error
	LoadErr     error
	IsRunning   bool
	IsRunningErr error
}

func (m *MockSessionManager) StartSession(sessionName string, command []string, workspace string) (*runner.SessionState, error) {
	return nil, nil
}

func (m *MockSessionManager) StopSession(sessionName string) error {
	return nil
}

func (m *MockSessionManager) AttachToSession(sessionName string) error {
	return nil
}

func (m *MockSessionManager) ReplaySession(sessionName, newSessionName string, iteration int) error {
	return nil
}

func (m *MockSessionManager) ListSessions() ([]*runner.SessionState, error) {
	if m.ListErr != nil {
		return nil, m.ListErr
	}
	return m.Sessions, nil
}

func (m *MockSessionManager) LoadSession(sessionName string) (*runner.SessionState, error) {
	if m.LoadErr != nil {
		return nil, m.LoadErr
	}
	for _, s := range m.Sessions {
		if s.Name == sessionName {
			return s, nil
		}
	}
	return nil, runner.ErrSessionNotFound
}

func (m *MockSessionManager) SaveSession(session *runner.SessionState) error {
	if m.SaveErr != nil {
		return m.SaveErr
	}
	m.Sessions = append(m.Sessions, session)
	return nil
}

func (m_ *MockSessionManager) IsProcessRunning(pid int) bool {
	return m_.IsRunning
}

func (m *MockSessionManager) GetSessionLog(sessionName string) (string, error) {
	return "", nil
}

func (m *MockSessionManager) GetSessionLogs(sessionName string) (string, error) {
	return "", nil
}

func (m *MockSessionManager) GetSessionLogStream(sessionName string) (<-chan string, error) {
	return nil, nil
}

func (m *MockSessionManager) GetSessionHistory(sessionName string) (string, error) {
	return "", nil
}

func (m *MockSessionManager) DeleteSession(sessionName string) error {
	return nil
}

func (m *MockSessionManager) UpdateSessionStatus(sessionName, status string) error {
	return nil
}

func (m *MockSessionManager) GetLastSessionName() (string, error) {
	return "", nil
}

func (m *MockSessionManager) GetSessionState(sessionName string) (*runner.SessionState, error) {
	return nil, nil
}

func (m *MockSessionManager) GetSessionStateFile(sessionName string) (string, error) {
	return "", nil
}

func (m *MockSessionManager) GetSessionDir(sessionName string) (string, error) {
	return "", nil
}

func (m *MockSessionManager) CreateSessionDir(sessionName string) (string, error) {
	return "", nil
}

func (m *MockSessionManager) CreateSessionLogFile(sessionName string) (string, error) {
	return "", nil
}

func (m *MockSessionManager) CreateSessionStateFile(sessionName string) (string, error) {
	return "", nil
}

func (m *MockSessionManager) SetSessionPID(sessionName string, pid int) error {
	return nil
}

func (m *MockSessionManager) GetSessionPID(sessionName string) (int, error) {
	return 0, nil
}

func (m *MockSessionManager) SetSessionEndTime(sessionName string, endTime time.Time) error {
	return nil
}

func (m *MockSessionManager) SetSessionError(sessionName string, err error) error {
	return nil
}
