package ui

import (
	"errors"
	"recac/internal/model"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestMonitorDashboardModel_Update_Refresh(t *testing.T) {
	callbacks := ActionCallbacks{}
	m := NewMonitorDashboardModel(callbacks)

	sessions := []model.UnifiedSession{
		{Name: "sess1", Status: "running", Location: "k8s", Goal: "test"},
	}
	msg := monitorSessionsRefreshedMsg(sessions)

	newM, _ := m.Update(msg)
	finalM := newM.(MonitorDashboardModel)

	assert.Len(t, finalM.sessions, 1)
	assert.Equal(t, "sess1", finalM.sessions[0].Name)
	// Table should have rows
	assert.NotEmpty(t, finalM.table.Rows())
}

func TestMonitorDashboardModel_Update_Kill(t *testing.T) {
	callbacks := ActionCallbacks{}
	m := NewMonitorDashboardModel(callbacks)

	// Populate table
	sessions := []model.UnifiedSession{
		{Name: "sess1", Status: "running"},
	}
	m, _ = updateModelWithSessions(m, sessions)

	// Select first row
	m.table.SetCursor(0)

	// Press 'k'
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}
	newM, _ := m.Update(msg)
	finalM := newM.(MonitorDashboardModel)

	assert.Equal(t, "confirm_kill", finalM.viewMode)
	assert.Equal(t, "sess1", finalM.sessionToKill)

	// Press 'y' to confirm
	mockStop := func(name string) error {
		assert.Equal(t, "sess1", name)
		return nil
	}
	finalM.callbacks.Stop = mockStop

	msgY := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}
	newM, cmd := finalM.Update(msgY)

	// Execute cmd
	resMsg := cmd()
	assert.IsType(t, actionResultMsg{}, resMsg)
	assert.Contains(t, resMsg.(actionResultMsg).msg, "Stopped session sess1")

	finalM = newM.(MonitorDashboardModel)
	assert.Equal(t, "list", finalM.viewMode)
}

func TestMonitorDashboardModel_Update_Logs(t *testing.T) {
	mockGetLogs := func(name string) (string, error) {
		return "log content", nil
	}
	callbacks := ActionCallbacks{GetLogs: mockGetLogs}
	m := NewMonitorDashboardModel(callbacks)

	// Populate table
	sessions := []model.UnifiedSession{
		{Name: "sess1"},
	}
	m, _ = updateModelWithSessions(m, sessions)

	// Press 'l'
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}}
	newM, cmd := m.Update(msg)

	// Cmd returns a function that returns a string
	resMsg := cmd()

	assert.Equal(t, "log content", resMsg)

	// Update with log content
	newM, _ = newM.Update(resMsg)
	finalM := newM.(MonitorDashboardModel)

	assert.Equal(t, "logs", finalM.viewMode)
	assert.Equal(t, "log content", finalM.logContent)
}

func TestMonitorDashboardModel_Update_PauseResume(t *testing.T) {
	mockPause := func(name string) error { return nil }
	mockResume := func(name string) error { return nil }

	callbacks := ActionCallbacks{Pause: mockPause, Resume: mockResume}
	m := NewMonitorDashboardModel(callbacks)

	// Populate table with running session
	sessions := []model.UnifiedSession{
		{Name: "sess1", Status: "running"},
	}
	m, _ = updateModelWithSessions(m, sessions)

	// Press 'p' -> Pause
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}}
	_, cmd := m.Update(msg)
	resMsg := cmd()
	assert.Contains(t, resMsg.(actionResultMsg).msg, "Paused session sess1")

	// Populate table with paused session
	sessions[0].Status = "paused"
	m, _ = updateModelWithSessions(m, sessions)

	// Press 'p' -> Resume
	_, cmd = m.Update(msg)
	resMsg = cmd()
	assert.Contains(t, resMsg.(actionResultMsg).msg, "Resumed session sess1")
}

func TestMonitorDashboardModel_View(t *testing.T) {
	m := NewMonitorDashboardModel(ActionCallbacks{})
	m, _ = updateModelWithSessions(m, []model.UnifiedSession{{Name: "sess1"}})

	// List view
	view := m.View()
	assert.Contains(t, view, "sess1")

	// Logs view
	m.viewMode = "logs"
	view = m.View()
	assert.Contains(t, view, "Session Logs")

	// Confirm Kill view
	m.viewMode = "confirm_kill"
	m.sessionToKill = "sess1"
	view = m.View()
	assert.Contains(t, view, "Are you sure you want to kill session 'sess1'?")
}

// Helper
func updateModelWithSessions(m MonitorDashboardModel, sessions []model.UnifiedSession) (MonitorDashboardModel, tea.Cmd) {
	newM, cmd := m.Update(monitorSessionsRefreshedMsg(sessions))
	return newM.(MonitorDashboardModel), cmd
}

func TestRefreshMonitorSessionsCmd(t *testing.T) {
	mockGet := func() ([]model.UnifiedSession, error) {
		return []model.UnifiedSession{{Name: "test"}}, nil
	}
	cmd := refreshMonitorSessionsCmd(mockGet)
	msg := cmd()
	assert.IsType(t, monitorSessionsRefreshedMsg{}, msg)
	assert.Len(t, msg.(monitorSessionsRefreshedMsg), 1)

	mockErr := func() ([]model.UnifiedSession, error) {
		return nil, errors.New("fail")
	}
	cmdErr := refreshMonitorSessionsCmd(mockErr)
	msgErr := cmdErr()
	assert.IsType(t, actionResultMsg{}, msgErr)
	assert.Error(t, msgErr.(actionResultMsg).err)
}

func TestMonitorDashboardModel_Visuals(t *testing.T) {
	// Setup
	callbacks := ActionCallbacks{}
	m := NewMonitorDashboardModel(callbacks)

	sessions := []model.UnifiedSession{
		{Name: "sess1", Status: "running", Location: "k8s", Goal: "test"},
		{Name: "sess2", Status: "paused", Location: "docker", Goal: "test"},
		{Name: "sess3", Status: "error", Location: "local", Goal: "test"},
	}

	// Update model
	m.sessions = sessions
	m.updateTableRows()

	// Check rows
	rows := m.table.Rows()
	assert.Len(t, rows, 3)

	// We expect icons in the status and location columns (index 1 and 2)
	// Note: We can't easily check for exact ANSI codes without being fragile,
	// but we can check for the presence of the emoji characters.

	// Row 1: Running, K8s
	assert.Contains(t, rows[0][1], "🟢") // Running
	assert.Contains(t, rows[0][2], "☸️")  // K8s

	// Row 2: Paused, Docker
	assert.Contains(t, rows[1][1], "⏸️") // Paused
	assert.Contains(t, rows[1][2], "🐳") // Docker

	// Row 3: Error, Local
	assert.Contains(t, rows[2][1], "❌") // Error
	assert.Contains(t, rows[2][2], "💻") // Local
}
