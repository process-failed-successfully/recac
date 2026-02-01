package ui

import (
	"errors"
	"recac/internal/model"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestNewMonitorDashboardModel(t *testing.T) {
	callbacks := ActionCallbacks{}
	m := NewMonitorDashboardModel(callbacks)
	assert.NotNil(t, m.table)
	assert.Equal(t, "list", m.viewMode)
}

func TestMonitorInit(t *testing.T) {
	callbacks := ActionCallbacks{
		GetSessions: func() ([]model.UnifiedSession, error) { return nil, nil },
	}
	m := NewMonitorDashboardModel(callbacks)
	cmd := m.Init()
	assert.NotNil(t, cmd)
}

func TestMonitorUpdate_Resize(t *testing.T) {
	m := NewMonitorDashboardModel(ActionCallbacks{})
	msg := tea.WindowSizeMsg{Width: 100, Height: 50}

	updatedM, _ := m.Update(msg)
	newM := updatedM.(MonitorDashboardModel)

	assert.Equal(t, 100, newM.width)
	assert.Equal(t, 50, newM.height)
}

func TestMonitorUpdate_Keys(t *testing.T) {
	// Setup callbacks
	stopCalled := false
	pauseCalled := false
	resumeCalled := false
	getLogsCalled := false

	callbacks := ActionCallbacks{
		Stop: func(name string) error { stopCalled = true; return nil },
		Pause: func(name string) error { pauseCalled = true; return nil },
		Resume: func(name string) error { resumeCalled = true; return nil },
		GetLogs: func(name string) (string, error) { getLogsCalled = true; return "logs", nil },
	}

	m := NewMonitorDashboardModel(callbacks)

	// Seed sessions
	m.sessions = []model.UnifiedSession{
		{Name: "sess1", Status: "running"},
		{Name: "sess2", Status: "paused"},
	}
	m.updateTableRows()
	// Select first row
	m.table.SetCursor(0)

	t.Run("Kill Flow", func(t *testing.T) {
		// Press k
		updatedM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
		newM := updatedM.(MonitorDashboardModel)
		assert.Equal(t, "confirm_kill", newM.viewMode)
		assert.Equal(t, "sess1", newM.sessionToKill)

		// Press n (cancel)
		updatedM, _ = newM.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
		newM = updatedM.(MonitorDashboardModel)
		assert.Equal(t, "list", newM.viewMode)
		assert.Equal(t, "", newM.sessionToKill)
		assert.False(t, stopCalled)

		// Press k again
		updatedM, _ = newM.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
		newM = updatedM.(MonitorDashboardModel)

		// Press y (confirm)
		updatedM, cmd := newM.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
		newM = updatedM.(MonitorDashboardModel)
		assert.Equal(t, "list", newM.viewMode)

		// Execute command to verify callback
		if cmd != nil {
			cmd()
		}
		assert.True(t, stopCalled)
	})

	t.Run("Pause/Resume", func(t *testing.T) {
		// Pause sess1 (running)
		m.table.SetCursor(0)
		updatedM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
		_ = updatedM
		if cmd != nil { cmd() }
		assert.True(t, pauseCalled)

		// Resume sess2 (paused)
		m.table.SetCursor(1)
		updatedM, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
		_ = updatedM
		if cmd != nil { cmd() }
		assert.True(t, resumeCalled)
	})

	t.Run("Logs", func(t *testing.T) {
		m.table.SetCursor(0)
		updatedM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
		// cmd fetches logs
		if cmd != nil {
			msg := cmd()
			// The cmd returns the string directly because of the hacky implementation
			if logMsg, ok := msg.(string); ok {
				updatedM, _ = updatedM.Update(logMsg)
			} else if fn, ok := msg.(func() tea.Msg); ok {
				// Just in case it was nested function, but based on code analysis it returns string
				logMsg := fn()
				updatedM, _ = updatedM.Update(logMsg)
			}
		}
		assert.True(t, getLogsCalled)
		newM := updatedM.(MonitorDashboardModel)
		assert.Equal(t, "logs", newM.viewMode)
		assert.Equal(t, "logs", newM.logContent)

		// Exit logs
		updatedM, _ = newM.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
		newM = updatedM.(MonitorDashboardModel)
		assert.Equal(t, "list", newM.viewMode)
	})
}

func TestMonitorUpdate_Messages(t *testing.T) {
	m := NewMonitorDashboardModel(ActionCallbacks{})

	// Refresh message
	sessions := []model.UnifiedSession{{Name: "s1"}}
	updatedM, _ := m.Update(monitorSessionsRefreshedMsg(sessions))
	newM := updatedM.(MonitorDashboardModel)
	assert.Len(t, newM.sessions, 1)

	// Action Result Error
	updatedM, _ = newM.Update(actionResultMsg{err: errors.New("fail")})
	newM = updatedM.(MonitorDashboardModel)
	assert.Contains(t, newM.message, "Error: fail")

	// Action Result Success
	updatedM, _ = newM.Update(actionResultMsg{msg: "success"})
	newM = updatedM.(MonitorDashboardModel)
	assert.Equal(t, "success", newM.message)
}

func TestMonitorView(t *testing.T) {
	m := NewMonitorDashboardModel(ActionCallbacks{})
	m.width = 80
	m.height = 24

	// List View
	assert.Contains(t, m.View(), "RECAC Control Center")

	// Logs View
	m.viewMode = "logs"
	assert.Contains(t, m.View(), "Session Logs")

	// Confirm Kill View
	m.viewMode = "confirm_kill"
	m.sessionToKill = "test"
	assert.Contains(t, m.View(), "Are you sure you want to kill session 'test'?")
}
