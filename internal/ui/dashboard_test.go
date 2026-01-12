package ui

import (
	"errors"
	"os"
	"recac/internal/runner"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
)

// MockDashboardSessionManager provides a mock implementation of the ISessionManager interface.
type MockDashboardSessionManager struct {
	Sessions []*runner.SessionState
	Error    error
}

func (m *MockDashboardSessionManager) ListSessions() ([]*runner.SessionState, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return m.Sessions, nil
}

func (m *MockDashboardSessionManager) GetSession(name string) (*runner.SessionState, error) {
	for _, s := range m.Sessions {
		if s.Name == name {
			return s, nil
		}
	}
	return nil, errors.New("session not found")
}

// updateModel is a helper to process a message and return the updated model.
func updateModel(m tea.Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	updatedModel, cmd := m.Update(msg)
	return updatedModel, cmd
}

func TestDashboardState_Initial(t *testing.T) {
	mockSM := &MockDashboardSessionManager{}
	model := NewDashboardModel(mockSM)
	require.True(t, model.loading)
}

func TestDashboardState_SessionsLoaded(t *testing.T) {
	sessions := []*runner.SessionState{
		{Name: "session-1", Status: "running", StartTime: time.Now()},
		{Name: "session-2", Status: "completed", StartTime: time.Now().Add(-time.Hour)},
	}
	mockSM := &MockDashboardSessionManager{Sessions: sessions}
	model := NewDashboardModel(mockSM)

	msg := sessionsRefreshedMsg{sessions: sessions}
	updatedModel, _ := updateModel(model, msg)
	model = updatedModel.(DashboardModel)

	require.False(t, model.loading)
	require.Len(t, model.sessions, 2)
	require.Len(t, model.sessionList.Items(), 2)
	require.Equal(t, "session-1", model.sessions[0].Name)
}

func TestDashboardState_UpdateQuit(t *testing.T) {
	mockSM := &MockDashboardSessionManager{}
	model := NewDashboardModel(mockSM)

	_, cmd := updateModel(model, tea.KeyMsg{Type: tea.KeyCtrlC})

	require.NotNil(t, cmd)
	quitMsg := cmd()
	require.NotNil(t, quitMsg)
	_, ok := quitMsg.(tea.QuitMsg)
	require.True(t, ok, "Expected a tea.QuitMsg")
}

func TestDashboardState_Empty(t *testing.T) {
	mockSM := &MockDashboardSessionManager{Sessions: []*runner.SessionState{}}
	model := NewDashboardModel(mockSM)

	msg := sessionsRefreshedMsg{sessions: []*runner.SessionState{}}
	updatedModel, _ := updateModel(model, msg)
	model = updatedModel.(DashboardModel)

	require.Len(t, model.sessions, 0)
	require.Len(t, model.sessionList.Items(), 0)
}

func TestDashboardState_Error(t *testing.T) {
	expectedErr := errors.New("failed to list sessions")
	mockSM := &MockDashboardSessionManager{Error: expectedErr}
	model := NewDashboardModel(mockSM)

	msg := refreshSessionsCmd(mockSM)()
	require.NotNil(t, msg, "should return a message even on error")

	updatedModel, _ := updateModel(model, msg)
	model = updatedModel.(DashboardModel)

	require.False(t, model.loading)
	require.Len(t, model.sessions, 0)
}

func TestDashboardState_Selection(t *testing.T) {
	tempDir := t.TempDir()
	logFile, err := os.CreateTemp(tempDir, "testlog-*.log")
	require.NoError(t, err)
	logFile.WriteString("this is a log line")
	logFile.Close()

	sessions := []*runner.SessionState{
		{Name: "session-1", Status: "completed", LogFile: logFile.Name()},
	}
	mockSM := &MockDashboardSessionManager{Sessions: sessions}
	model := NewDashboardModel(mockSM)

	m, _ := updateModel(model, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = updateModel(m, sessionsRefreshedMsg{sessions: sessions})
	m, _ = updateModel(m, tea.KeyMsg{Type: tea.KeyDown})
	model = m.(DashboardModel)

	require.NotNil(t, model.sessionList.SelectedItem())
	selected, ok := model.sessionList.SelectedItem().(sessionItem)
	require.True(t, ok)
	require.Equal(t, "session-1", selected.name)

	// Manually trigger the details view update for state check
	model.updateDetailsView()
	require.Contains(t, model.detailsView.View(), "Name: session-1")
	require.Contains(t, model.logView.View(), "this is a log line")
}