package ui

import (
	"errors"
	"recac/internal/model"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockSessionManager is a mock implementation of ISessionManagerDashboard for testing.
type MockSessionManager struct {
	mock.Mock
}

func (m *MockSessionManager) StopSession(name string) error {
	args := m.Called(name)
	return args.Error(0)
}

func (m *MockSessionManager) ArchiveSession(name string) error {
	args := m.Called(name)
	return args.Error(0)
}

func (m *MockSessionManager) GetSessionLogs(name string) (string, error) {
	args := m.Called(name)
	return args.String(0), args.Error(1)
}

func (m *MockSessionManager) GetSessionDiff(name string) (string, error) {
	args := m.Called(name)
	return args.String(0), args.Error(1)
}

func TestPsDashboard_StateTransitions(t *testing.T) {
	mockSM := new(MockSessionManager)
	m := NewPsDashboardModel(mockSM)

	// Initial state should be listView
	assert.Equal(t, listView, m.state)

	// Simulate receiving sessions
	sessions := []model.UnifiedSession{
		{Name: "test-session-1", Status: "running"},
		{Name: "test-session-2", Status: "completed"},
	}
	model, _ := m.Update(psSessionsRefreshedMsg(sessions))
	m = model.(psDashboardModel)

	// Press Enter to select a session and switch to detailView
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(psDashboardModel)
	assert.Equal(t, detailView, m.state)
	assert.NotNil(t, m.selectedSession)
	assert.Equal(t, "test-session-1", m.selectedSession.Name)

	// Press Backspace to go back to listView
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m = model.(psDashboardModel)
	assert.Equal(t, listView, m.state)
	assert.Nil(t, m.selectedSession)
}

func TestPsDashboard_Actions(t *testing.T) {
	mockSM := new(MockSessionManager)
	m := NewPsDashboardModel(mockSM)

	sessions := []model.UnifiedSession{{Name: "test-session", Status: "running"}}
	model, _ := m.Update(psSessionsRefreshedMsg(sessions))
	m = model.(psDashboardModel)

	// --- Enter Detail View ---
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(psDashboardModel)
	assert.Equal(t, detailView, m.state)

	// --- Test Stop Action ---
	mockSM.On("StopSession", "test-session").Return(nil).Once()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	msg := cmd()
	assert.IsType(t, notificationMsg(""), msg)
	assert.Equal(t, "Session 'test-session' stopped.", string(msg.(notificationMsg)))
	mockSM.AssertExpectations(t)

	// --- Test Archive Action ---
	mockSM.On("ArchiveSession", "test-session").Return(nil).Once()
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	msg = cmd()
	assert.IsType(t, notificationMsg(""), msg)
	assert.Equal(t, "Session 'test-session' archived.", string(msg.(notificationMsg)))
	mockSM.AssertExpectations(t)

	// --- Test Archive Action (Error) ---
	mockSM.On("ArchiveSession", "test-session").Return(errors.New("archive error")).Once()
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	msg = cmd()
	assert.IsType(t, notificationMsg(""), msg)
	assert.Contains(t, string(msg.(notificationMsg)), "Error archiving session: archive error")
	mockSM.AssertExpectations(t)
}

func TestPsDashboard_DataFetching(t *testing.T) {
	mockSM := new(MockSessionManager)
	m := NewPsDashboardModel(mockSM)

	sessions := []model.UnifiedSession{{Name: "data-session"}}
	model, _ := m.Update(psSessionsRefreshedMsg(sessions))
	m = model.(psDashboardModel)

	// --- Enter Detail View ---
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(psDashboardModel)
	assert.Equal(t, detailView, m.state)
	assert.Equal(t, "", m.detailContent)

	// --- Test Logs Fetching ---
	expectedLogs := "This is the log content."
	mockSM.On("GetSessionLogs", "data-session").Return(expectedLogs, nil).Once()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	msg := cmd()
	model, _ = m.Update(msg) // Process the returned message
	m = model.(psDashboardModel)
	assert.Equal(t, expectedLogs, m.detailContent)
	mockSM.AssertExpectations(t)

	// --- First Back press should clear content ---
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m = model.(psDashboardModel)
	assert.Equal(t, "", m.detailContent)
	assert.Equal(t, detailView, m.state) // Still in detail view

	// --- Test Diff Fetching ---
	expectedDiff := "--- a/file.go\n+++ b/file.go"
	mockSM.On("GetSessionDiff", "data-session").Return(expectedDiff, nil).Once()
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	msg = cmd()
	model, _ = m.Update(msg)
	m = model.(psDashboardModel)
	assert.Equal(t, expectedDiff, m.detailContent)
	mockSM.AssertExpectations(t)

	// --- Second Back press should go to list view ---
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace}) // Clear content
	m = model.(psDashboardModel)
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace}) // Go back
	m = model.(psDashboardModel)
	assert.Equal(t, listView, m.state)
}
