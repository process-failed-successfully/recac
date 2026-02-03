package ui

import (
	"errors"
	"recac/internal/model"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestNewMonitorDashboardModel(t *testing.T) {
	callbacks := ActionCallbacks{}
	m := NewMonitorDashboardModel(callbacks)

	assert.Equal(t, "list", m.viewMode)
	assert.Empty(t, m.sessions)
	assert.Equal(t, 20, m.table.Columns()[0].Width)
}

func TestMonitorDashboardModel_Update_Refresh(t *testing.T) {
	callbacks := ActionCallbacks{}
	m := NewMonitorDashboardModel(callbacks)

	sessions := []model.UnifiedSession{
		{Name: "S1", Status: "running", HasCost: true, Cost: 1.23},
	}
	msg := monitorSessionsRefreshedMsg(sessions)

	updatedModel, cmd := m.Update(msg)
	newM := updatedModel.(MonitorDashboardModel)

	assert.Len(t, newM.sessions, 1)
	assert.Equal(t, "S1", newM.sessions[0].Name)
	assert.Nil(t, cmd) // No command returned on refresh update (other than maybe table update internal)

	// Check table rows
	rows := newM.table.Rows()
	assert.Len(t, rows, 1)
	assert.Equal(t, "S1", rows[0][0])
	assert.Equal(t, "$1.2300", rows[0][3])
}

func TestMonitorDashboardModel_Update_Kill(t *testing.T) {
	stoppedName := ""
	callbacks := ActionCallbacks{
		Stop: func(name string) error {
			stoppedName = name
			return nil
		},
	}
	m := NewMonitorDashboardModel(callbacks)
	m.sessions = []model.UnifiedSession{{Name: "S1"}}
	m.updateTableRows()
	m.table.SetCursor(0)

	// 1. Press 'k' to enter confirm mode
	updatedModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	newM := updatedModel.(MonitorDashboardModel)
	assert.Equal(t, "confirm_kill", newM.viewMode)
	assert.Equal(t, "S1", newM.sessionToKill)

	// 2. Press 'y' to confirm
	updatedModel, cmd := newM.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	newM = updatedModel.(MonitorDashboardModel)
	assert.Equal(t, "list", newM.viewMode)
	assert.Empty(t, newM.sessionToKill)

	// execute the command
	if cmd != nil {
		msg := cmd()
		res, ok := msg.(actionResultMsg)
		assert.True(t, ok)
		assert.Contains(t, res.msg, "Stopped session S1")
		assert.Equal(t, "S1", stoppedName)
	} else {
		t.Error("Expected command")
	}
}

func TestMonitorDashboardModel_Update_Logs(t *testing.T) {
	callbacks := ActionCallbacks{
		GetLogs: func(name string) (string, error) {
			return "some logs", nil
		},
	}
	m := NewMonitorDashboardModel(callbacks)
	m.sessions = []model.UnifiedSession{{Name: "S1"}}
	m.updateTableRows()
	m.table.SetCursor(0)
	m.viewport.Height = 10
	m.viewport.Width = 20

	// Press 'l'
	updatedModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	newM := updatedModel.(MonitorDashboardModel)
	// View mode doesn't change immediately, it changes when log content msg is received

	if cmd != nil {
		// nested command: fetches logs then returns the log string directly (because of the immediate invocation in Update)
		msg := cmd()
		logMsg, ok := msg.(string)
		if !ok {
			t.Fatalf("Expected string message, got %T: %v", msg, msg)
		}
		if logMsg != "some logs" {
			t.Errorf("Expected 'some logs', got '%s'", logMsg)
		}

		// Update with log content
		updatedModel, _ = newM.Update(logMsg)
		newM = updatedModel.(MonitorDashboardModel)
		assert.Equal(t, "logs", newM.viewMode)
		assert.Contains(t, newM.viewport.View(), "some logs")
	} else {
		t.Error("Expected command")
	}
}

func TestMonitorDashboardModel_Update_PauseResume(t *testing.T) {
	pausedName := ""
	callbacks := ActionCallbacks{
		Pause: func(name string) error {
			pausedName = name
			return nil
		},
	}
	m := NewMonitorDashboardModel(callbacks)
	m.sessions = []model.UnifiedSession{{Name: "S1", Status: "running"}}
	m.updateTableRows()

	// Press 'p'
	updatedModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	_ = updatedModel.(MonitorDashboardModel)

	if cmd != nil {
		msg := cmd()
		res, ok := msg.(actionResultMsg)
		assert.True(t, ok)
		assert.Contains(t, res.msg, "Paused session S1")
		assert.Equal(t, "S1", pausedName)
	}
}

func TestMonitorDashboardModel_View(t *testing.T) {
	m := NewMonitorDashboardModel(ActionCallbacks{})
	// Basic check that it renders something
	assert.Contains(t, m.View(), "RECAC Control Center")
	assert.Contains(t, m.View(), "No active sessions")
}

func TestRefreshMonitorSessionsCmd(t *testing.T) {
	// Success case
	cmd := refreshMonitorSessionsCmd(func() ([]model.UnifiedSession, error) {
		return []model.UnifiedSession{{Name: "A"}}, nil
	})
	msg := cmd()
	sessions, ok := msg.(monitorSessionsRefreshedMsg)
	assert.True(t, ok)
	assert.Len(t, sessions, 1)

	// Error case
	cmdErr := refreshMonitorSessionsCmd(func() ([]model.UnifiedSession, error) {
		return nil, errors.New("fail")
	})
	msgErr := cmdErr()
	res, ok := msgErr.(actionResultMsg)
	assert.True(t, ok)
	assert.Error(t, res.err)
}

func TestMonitorDashboardModel_Tick(t *testing.T) {
	m := NewMonitorDashboardModel(ActionCallbacks{GetSessions: func() ([]model.UnifiedSession, error) { return nil, nil }})

	// Trigger tick
	_, cmd := m.Update(monitorTickMsg(time.Now()))
	assert.NotNil(t, cmd)
	// Verify it returns a batch (refresh + next tick)
}
