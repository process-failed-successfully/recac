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

func TestMonitorDashboardModel_Update_Refresh(t *testing.T) {
	callbacks := ActionCallbacks{}
	m := NewMonitorDashboardModel(callbacks)

	sessions := []model.UnifiedSession{
		{Name: "session-1", Status: "running", Goal: "Fix bug"},
	}

	updatedM, cmd := m.Update(monitorSessionsRefreshedMsg(sessions))
	model := updatedM.(MonitorDashboardModel)

	assert.Nil(t, cmd)
	assert.Equal(t, 1, len(model.sessions))
	assert.Equal(t, "session-1", model.sessions[0].Name)

	// Check table rows
	rows := model.table.Rows()
	assert.Equal(t, 1, len(rows))
	assert.Equal(t, "session-1", rows[0][0])
}

func TestMonitorDashboardModel_Update_Kill(t *testing.T) {
	killed := ""
	callbacks := ActionCallbacks{
		Stop: func(name string) error {
			killed = name
			return nil
		},
	}
	m := NewMonitorDashboardModel(callbacks)
	m.sessions = []model.UnifiedSession{
		{Name: "session-to-kill"},
	}
	m.updateTableRows()
	m.table.SetCursor(0)

	// 1. Simulate 'k' key press -> Enter confirmation mode
	updatedM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	model := updatedM.(MonitorDashboardModel)

	assert.Nil(t, cmd)
	assert.Equal(t, "confirm_kill", model.viewMode)
	assert.Equal(t, "session-to-kill", model.sessionToKill)

	// 2. Simulate 'y' key press -> Execute kill
	updatedM, cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})

	// The cmd returned should be the function that calls Stop
	assert.NotNil(t, cmd)

	// Execute the cmd
	msg := cmd()
	assert.Equal(t, "session-to-kill", killed)

	// Check result message
	resMsg, ok := msg.(actionResultMsg)
	assert.True(t, ok)
	assert.Equal(t, "Stopped session session-to-kill", resMsg.msg)
	assert.NoError(t, resMsg.err)

	// Update model with result
	finalM, _ := updatedM.Update(msg)
	finalModel := finalM.(MonitorDashboardModel)
	assert.Equal(t, "Stopped session session-to-kill", finalModel.message)
	assert.Equal(t, "list", finalModel.viewMode)
}

func TestMonitorDashboardModel_Update_Kill_Cancel(t *testing.T) {
	m := NewMonitorDashboardModel(ActionCallbacks{})
	m.sessions = []model.UnifiedSession{{Name: "s1"}}
	m.updateTableRows()

	// 1. Press 'k'
	updatedM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	model := updatedM.(MonitorDashboardModel)
	assert.Equal(t, "confirm_kill", model.viewMode)

	// 2. Press 'n'
	updatedM, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	model = updatedM.(MonitorDashboardModel)

	assert.Equal(t, "list", model.viewMode)
	assert.Equal(t, "", model.sessionToKill)
}

func TestMonitorDashboardModel_Update_Kill_Error(t *testing.T) {
	callbacks := ActionCallbacks{
		Stop: func(name string) error {
			return errors.New("failed to stop")
		},
	}
	m := NewMonitorDashboardModel(callbacks)
	m.sessions = []model.UnifiedSession{{Name: "s1"}}
	m.updateTableRows()

	// 1. Press 'k'
	updatedM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	model := updatedM.(MonitorDashboardModel)

	// 2. Press 'y'
	updatedM, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	msg := cmd()

	resMsg, ok := msg.(actionResultMsg)
	assert.True(t, ok)
	assert.Error(t, resMsg.err)
	assert.Equal(t, "failed to stop", resMsg.err.Error())

	finalM, _ := updatedM.Update(msg)
	finalModel := finalM.(MonitorDashboardModel)
	assert.Contains(t, finalModel.message, "Error: failed to stop")
}

func TestMonitorDashboardModel_Update_Logs(t *testing.T) {
	callbacks := ActionCallbacks{
		GetLogs: func(name string) (string, error) {
			return "log content", nil
		},
	}
	m := NewMonitorDashboardModel(callbacks)
	m.sessions = []model.UnifiedSession{{Name: "s1"}}
	m.updateTableRows()

	// 1. Press 'l'
	updatedM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	assert.NotNil(t, cmd)

	// 2. Execute cmd -> returns string log content
	msg := cmd()
	logContent, ok := msg.(string)
	// Actually, the cmd returns a func() tea.Msg which returns the string?
	// Wait, in my impl: return m, func() tea.Msg { ... return func() tea.Msg { return logs }() }
	// The outer func returns the inner msg.
	// Let's trace carefully:
	// return m, func() tea.Msg {
    //    logs, _ := callbacks.GetLogs(name)
    //    return func() tea.Msg { return logs }()
    // }
    // So the tea.Msg is `logs` (string).

    // BUT wait, `func() tea.Msg { return logs }()` calls the func and returns `logs` (string).
    // So `msg` IS `string`.

    logStr, ok := msg.(string)
    // If it failed, maybe it's actionResultMsg (error)
    if !ok {
	// Check if it's error
	res, ok2 := msg.(actionResultMsg)
	if ok2 {
		t.Fatalf("Expected string logs, got error: %v", res.err)
	}
	// It might be that my test logic for cmd execution is slightly off regarding nested funcs?
	// No, `cmd()` executes the command function and returns `tea.Msg`.
	// My code: `return func() tea.Msg { return logs }()`. This is a direct return of `logs`.
	// So `msg` is `logs`.
    }
    assert.True(t, ok)
    assert.Equal(t, "log content", logStr)

    // 3. Update model with logs
    finalM, _ := updatedM.Update(logContent)
    finalModel := finalM.(MonitorDashboardModel)

    assert.Equal(t, "logs", finalModel.viewMode)
    assert.Equal(t, "log content", finalModel.logContent)
    assert.Contains(t, finalModel.View(), "Session Logs")

    // 4. Press 'q' to go back
    backM, _ := finalM.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
    backModel := backM.(MonitorDashboardModel)
    assert.Equal(t, "list", backModel.viewMode)
}

func TestMonitorDashboardModel_Update_Pause(t *testing.T) {
	paused := ""
	resumed := ""
	callbacks := ActionCallbacks{
		Pause: func(name string) error {
			paused = name
			return nil
		},
		Resume: func(name string) error {
			resumed = name
			return nil
		},
	}
	m := NewMonitorDashboardModel(callbacks)

	// Case 1: Pause running session
	m.sessions = []model.UnifiedSession{{Name: "s1", Status: "running"}}
	m.updateTableRows()
	m.table.SetCursor(0)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	msg := cmd()
	assert.Equal(t, "s1", paused)
	resMsg, ok := msg.(actionResultMsg)
	assert.True(t, ok)
	assert.Equal(t, "Paused session s1", resMsg.msg)

	// Case 2: Resume paused session
	m.sessions = []model.UnifiedSession{{Name: "s1", Status: "paused"}}
	m.updateTableRows()
	m.table.SetCursor(0)

	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	msg = cmd()
	assert.Equal(t, "s1", resumed)
	resMsg, ok = msg.(actionResultMsg)
	assert.True(t, ok)
	assert.Equal(t, "Resumed session s1", resMsg.msg)

	// Case 3: Invalid status
	m.sessions = []model.UnifiedSession{{Name: "s1", Status: "stopped"}}
	m.updateTableRows()
	m.table.SetCursor(0)
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	msg = cmd()
	resMsg, ok = msg.(actionResultMsg)
	assert.True(t, ok)
	assert.Error(t, resMsg.err)
}

func TestMonitorDashboardModel_Update_Resize(t *testing.T) {
	m := NewMonitorDashboardModel(ActionCallbacks{})
	updatedM, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	model := updatedM.(MonitorDashboardModel)

	// Height - 10 for table (minus borders = 38)
	assert.Equal(t, 38, model.table.Height())
	// Height - 5 for viewport
	assert.Equal(t, 45, model.viewport.Height)
}

func TestMonitorDashboardModel_View(t *testing.T) {
	m := NewMonitorDashboardModel(ActionCallbacks{})
	// Set message
	m.message = "Test Message"
	view := m.View()
	assert.Contains(t, view, "RECAC Control Center")
	assert.Contains(t, view, "Test Message")
}

func TestMonitorDashboardModel_View_EmptyState(t *testing.T) {
	m := NewMonitorDashboardModel(ActionCallbacks{})
	m.sessions = []model.UnifiedSession{}
	view := m.View()
	assert.Contains(t, view, "No active sessions found")
	assert.Contains(t, view, "recac start")
}

func TestMonitorDashboardModel_View_ConfirmKill(t *testing.T) {
	m := NewMonitorDashboardModel(ActionCallbacks{})
	m.viewMode = "confirm_kill"
	m.sessionToKill = "bad-session"
	view := m.View()
	assert.Contains(t, view, "DANGER ZONE")
	assert.Contains(t, view, "bad-session")
	assert.Contains(t, view, "(y/n)")
}

func TestMonitorDashboardModel_Update_Logs_Navigation(t *testing.T) {
	m := NewMonitorDashboardModel(ActionCallbacks{})
	m.viewMode = "logs"
	m.logContent = "logs"

	// 1. Press 'esc' to go back
	updatedM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model := updatedM.(MonitorDashboardModel)
	assert.Equal(t, "list", model.viewMode)

	// Reset
	m.viewMode = "logs"
	// 2. Press 'q' to go back
	updatedM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	model = updatedM.(MonitorDashboardModel)
	assert.Equal(t, "list", model.viewMode)
}

func TestMonitorDashboardModel_Update_Kill_Confirm_Navigation(t *testing.T) {
	m := NewMonitorDashboardModel(ActionCallbacks{})
	m.viewMode = "confirm_kill"
	m.sessionToKill = "s1"

	// 1. Press 'esc'
	updatedM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model := updatedM.(MonitorDashboardModel)
	assert.Equal(t, "list", model.viewMode)
	assert.Equal(t, "", model.sessionToKill)

	// Reset
	m.viewMode = "confirm_kill"
	m.sessionToKill = "s1"
	// 2. Press 'q'
	updatedM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	model = updatedM.(MonitorDashboardModel)
	assert.Equal(t, "list", model.viewMode)
}
