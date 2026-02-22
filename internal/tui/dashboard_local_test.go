package tui

import (
	"recac/internal/runner"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockSessionManager implements runner.ISessionManager for testing
type MockSessionManager struct {
	mock.Mock
}

func (m *MockSessionManager) ListSessions() ([]*runner.SessionState, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*runner.SessionState), args.Error(1)
}

func (m *MockSessionManager) GetSessionLogContent(name string, lines int) (string, error) {
	args := m.Called(name, lines)
	return args.String(0), args.Error(1)
}

// Stubs for unused methods
func (m *MockSessionManager) SaveSession(s *runner.SessionState) error { return nil }
func (m *MockSessionManager) LoadSession(name string) (*runner.SessionState, error) { return nil, nil }
func (m *MockSessionManager) StopSession(name string) error { return nil }
func (m *MockSessionManager) PauseSession(name string) error { return nil }
func (m *MockSessionManager) ResumeSession(name string) error { return nil }
func (m *MockSessionManager) GetSessionLogs(name string) (string, error) { return "", nil }
func (m *MockSessionManager) StartSession(name, goal string, command []string, workspace string) (*runner.SessionState, error) { return nil, nil }
func (m *MockSessionManager) GetSessionPath(name string) string { return "" }
func (m *MockSessionManager) IsProcessRunning(pid int) bool { return false }
func (m *MockSessionManager) RemoveSession(name string, force bool) error { return nil }
func (m *MockSessionManager) RenameSession(oldName, newName string) error { return nil }
func (m *MockSessionManager) SessionsDir() string { return "" }
func (m *MockSessionManager) GetSessionGitDiffStat(name string) (string, error) { return "", nil }
func (m *MockSessionManager) ArchiveSession(name string) error { return nil }
func (m *MockSessionManager) UnarchiveSession(name string) error { return nil }
func (m *MockSessionManager) ListArchivedSessions() ([]*runner.SessionState, error) { return nil, nil }

func TestLocalDashboardModel_Init(t *testing.T) {
	sm := new(MockSessionManager)
	model := NewLocalDashboardModel(sm)

	cmd := model.Init()
	assert.NotNil(t, cmd)
}

func TestLocalDashboardModel_Update_Tick(t *testing.T) {
	sm := new(MockSessionManager)
	sessions := []*runner.SessionState{
		{Name: "session-1", Status: "running", StartTime: time.Now()},
	}
	sm.On("ListSessions").Return(sessions, nil)

	model := NewLocalDashboardModel(sm)

	// Simulate tick
	newModel, cmd := model.Update(localTickMsg(time.Now()))

	// Should trigger fetchSessionsCmd
	assert.NotNil(t, cmd)

	m, ok := newModel.(LocalDashboardModel)
	assert.True(t, ok)

	// Now simulate the result of fetchSessionsCmd (localSessionListMsg)
	newModel, _ = m.Update(localSessionListMsg(sessions))
	m = newModel.(LocalDashboardModel)

	assert.Len(t, m.sessions, 1)
	assert.Equal(t, "session-1", m.sessions[0].Name)
}

func TestLocalDashboardModel_Update_SelectSession(t *testing.T) {
	sm := new(MockSessionManager)
	sessions := []*runner.SessionState{
		{Name: "session-1", Status: "running"},
	}
	// Setup ListSessions for initial load
	sm.On("ListSessions").Return(sessions, nil)
	// Setup logs fetch
	sm.On("GetSessionLogContent", "session-1", 1000).Return("some logs", nil)

	model := NewLocalDashboardModel(sm)

	// Load sessions
	newModel, _ := model.Update(localSessionListMsg(sessions))
	m := newModel.(LocalDashboardModel)

	// Simulate Enter key on list
	// We need to set window size first so list is initialized
	newModel, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	m = newModel.(LocalDashboardModel)

	// Select first item (default) and press enter
	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(LocalDashboardModel)

	assert.NotNil(t, cmd) // Should return fetchLogsCmd
	assert.Equal(t, "session-1", m.selected.Name)

	// Simulate logs loaded
	newModel, _ = m.Update(localLogsMsg("loaded logs"))
	m = newModel.(LocalDashboardModel)

	assert.Equal(t, "loaded logs", m.logs)
}
