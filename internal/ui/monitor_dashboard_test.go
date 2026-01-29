package ui

import (
	"errors"
	"recac/internal/model"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestMonitorDashboardModel_Init(t *testing.T) {
	callbacks := ActionCallbacks{
		GetSessions: func() ([]model.UnifiedSession, error) {
			return []model.UnifiedSession{}, nil
		},
	}
	m := NewMonitorDashboardModel(callbacks)
	cmd := m.Init()
	assert.NotNil(t, cmd)
}

func TestMonitorDashboardModel_Update(t *testing.T) {
	// Setup callbacks
	sessions := []model.UnifiedSession{
		{Name: "session-1", Status: "running", Goal: "Goal 1"},
		{Name: "session-2", Status: "paused", Goal: "Goal 2"},
	}

	callbacks := ActionCallbacks{
		GetSessions: func() ([]model.UnifiedSession, error) {
			return sessions, nil
		},
		Stop: func(name string) error {
			if name == "session-1" {
				return nil
			}
			return errors.New("stop failed")
		},
		Pause: func(name string) error {
			return nil
		},
		Resume: func(name string) error {
			return nil
		},
		GetLogs: func(name string) (string, error) {
			return "logs", nil
		},
	}

	m := NewMonitorDashboardModel(callbacks)
	m.width = 100
	m.height = 50
	m.sessions = sessions
	m.updateTableRows()

	// 1. Test WindowSizeMsg
	newModel, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = newModel.(MonitorDashboardModel)
	assert.Equal(t, 80, m.width)
	assert.Equal(t, 24, m.height)

	// 2. Test Refresh Msg
	newModel, _ = m.Update(monitorSessionsRefreshedMsg(sessions))
	m = newModel.(MonitorDashboardModel)
	assert.Equal(t, 2, len(m.sessions))

	// 3. Test Action Result
	newModel, cmd := m.Update(actionResultMsg{msg: "Action done"})
	m = newModel.(MonitorDashboardModel)
	assert.Equal(t, "Action done", m.message)
	assert.NotNil(t, cmd) // Should return a tick to clear message

	// 4. Test Key 'k' (Kill) -> Confirm Mode
	m.table.SetCursor(0) // Select first row
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = newModel.(MonitorDashboardModel)
	assert.Equal(t, "confirm_kill", m.viewMode)
	assert.Equal(t, "session-1", m.sessionToKill)

	// 5. Test Confirm Kill 'y'
	newModel, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = newModel.(MonitorDashboardModel)
	assert.Equal(t, "list", m.viewMode)
	assert.Equal(t, "", m.sessionToKill)
	// Execute command
	msg := cmd()
	res, ok := msg.(actionResultMsg)
	assert.True(t, ok)
	assert.Contains(t, res.msg, "Stopped session session-1")

	// 6. Test Logs 'l'
	m.viewMode = "list"
	newModel, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	// Result is a command that fetches logs
	msg = cmd()
	// The command returns a function that returns the log string
	// Wait, code says: return func() tea.Msg { return logs }()
	// So msg IS the string logs.
	assert.Equal(t, "logs", msg)

	// 7. Test Log Content Msg (string)
	newModel, _ = m.Update("log content")
	m = newModel.(MonitorDashboardModel)
	assert.Equal(t, "logs", m.viewMode)
	assert.Equal(t, "log content", m.logContent)

	// 8. Test Esc from Logs
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newModel.(MonitorDashboardModel)
	assert.Equal(t, "list", m.viewMode)

	// 9. Test Pause 'p'
	m.table.SetCursor(0) // session-1 running
	newModel, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	msg = cmd()
	res, ok = msg.(actionResultMsg)
	assert.True(t, ok)
	assert.Contains(t, res.msg, "Paused session session-1")

	// 10. Test Resume 'p'
	m.table.SetCursor(1) // session-2 paused
	newModel, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	msg = cmd()
	res, ok = msg.(actionResultMsg)
	assert.True(t, ok)
	assert.Contains(t, res.msg, "Resumed session session-2")
}

func TestMonitorDashboardModel_View(t *testing.T) {
	m := NewMonitorDashboardModel(ActionCallbacks{})
	m.sessions = []model.UnifiedSession{
		{Name: "test", Status: "running", Goal: "test goal", HasCost: true, Cost: 1.23},
	}
	m.updateTableRows()

	m.width = 100
	m.height = 50
	m.viewport.Width = 100
	m.viewport.Height = 40

	// List View
	m.viewMode = "list"
	assert.Contains(t, m.View(), "RECAC Control Center")
	assert.Contains(t, m.View(), "test")
	assert.Contains(t, m.View(), "$1.2300")

	// Logs View
	m.viewMode = "logs"
	m.logContent = "some logs"
	m.viewport.SetContent("some logs")
	assert.Contains(t, m.View(), "Session Logs")
	assert.Contains(t, m.View(), "some logs")

	// Confirm Kill View
	m.viewMode = "confirm_kill"
	m.sessionToKill = "test"
	assert.Contains(t, m.View(), "DANGER ZONE")
	assert.Contains(t, m.View(), "kill session 'test'?")
}

func TestRefreshMonitorSessionsCmd(t *testing.T) {
	// Success case
	cmd := refreshMonitorSessionsCmd(func() ([]model.UnifiedSession, error) {
		return []model.UnifiedSession{{Name: "A"}}, nil
	})
	msg := cmd()
	assert.IsType(t, monitorSessionsRefreshedMsg{}, msg)
	assert.Len(t, msg.(monitorSessionsRefreshedMsg), 1)

	// Error case
	cmd = refreshMonitorSessionsCmd(func() ([]model.UnifiedSession, error) {
		return nil, errors.New("fail")
	})
	msg = cmd()
	assert.IsType(t, actionResultMsg{}, msg)
	assert.Error(t, msg.(actionResultMsg).err)
}
