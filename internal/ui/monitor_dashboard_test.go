package ui

import (
	"errors"
	"recac/internal/model"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestMonitorDashboardModel_Init(t *testing.T) {
	callbacks := ActionCallbacks{
		GetSessions: func() ([]model.UnifiedSession, error) { return nil, nil },
	}
	m := NewMonitorDashboardModel(callbacks)
	cmd := m.Init()
	assert.NotNil(t, cmd)
}

func TestMonitorDashboardModel_Update_Refresh(t *testing.T) {
	callbacks := ActionCallbacks{
		GetSessions: func() ([]model.UnifiedSession, error) { return nil, nil },
	}
	m := NewMonitorDashboardModel(callbacks)

	// Simulate session refresh
	sessions := []model.UnifiedSession{
		{Name: "session1", Status: "running", Location: "local", Cost: 1.23, Goal: "Test Goal"},
	}

	updatedM, cmd := m.Update(monitorSessionsRefreshedMsg(sessions))
	m = updatedM.(MonitorDashboardModel)

	assert.Nil(t, cmd) // UpdateTableRows returns no cmd
	assert.Equal(t, sessions, m.sessions)
	assert.False(t, m.lastUpdate.IsZero())

	// Verify table rows (indirectly via sessions)
	assert.Equal(t, 1, len(m.sessions))
}

func TestMonitorDashboardModel_Update_Tick(t *testing.T) {
	called := false
	callbacks := ActionCallbacks{
		GetSessions: func() ([]model.UnifiedSession, error) {
			called = true
			return nil, nil
		},
	}
	m := NewMonitorDashboardModel(callbacks)

	updatedM, cmd := m.Update(monitorTickMsg(time.Now()))
	m = updatedM.(MonitorDashboardModel)
	assert.NotNil(t, cmd)

	// Execute cmd to verify it triggers refresh
	// Cmd is a Batch of refresh + tick.
	// We can't easily check batch content, but we can execute it.
	// If it contains the refresh cmd, it will call GetSessions.
	// However, tea.Batch returns a single Cmd that returns a tea.BatchMsg or executes multiple cmds.
	// Actually tea.Batch returns a Cmd that executes sub-commands.
	// We can't easily inspect it. But we trust `Init` and `monitorTickMsg` logic uses `refreshMonitorSessionsCmd`.

	// Let's verify that refreshMonitorSessionsCmd calls the callback.
	refreshCmd := refreshMonitorSessionsCmd(callbacks.GetSessions)
	msg := refreshCmd()
	assert.True(t, called)
	assert.IsType(t, monitorSessionsRefreshedMsg{}, msg)
}

func TestMonitorDashboardModel_Update_Keys(t *testing.T) {
	// Setup sessions
	sessions := []model.UnifiedSession{
		{Name: "session1", Status: "running"},
		{Name: "session2", Status: "paused"},
	}

	var stopCalled, pauseCalled, resumeCalled bool
	var stopName, pauseName, resumeName string

	callbacks := ActionCallbacks{
		Stop: func(name string) error {
			stopCalled = true
			stopName = name
			return nil
		},
		Pause: func(name string) error {
			pauseCalled = true
			pauseName = name
			return nil
		},
		Resume: func(name string) error {
			resumeCalled = true
			resumeName = name
			return nil
		},
		GetLogs: func(name string) (string, error) {
			return "logs for " + name, nil
		},
	}

	m := NewMonitorDashboardModel(callbacks)
	m.sessions = sessions
	m.updateTableRows()

	// Select first row (session1, running)
	m.table.SetCursor(0)

	// Test Pause (p)
	updatedM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	m = updatedM.(MonitorDashboardModel)
	assert.NotNil(t, cmd)

	// Execute cmd
	msg := cmd()
	assert.True(t, pauseCalled)
	assert.Equal(t, "session1", pauseName)
	assert.IsType(t, actionResultMsg{}, msg)
	assert.Contains(t, msg.(actionResultMsg).msg, "Paused session session1")

	// Select second row (session2, paused)
	m.table.SetCursor(1)

	// Test Resume (p)
	updatedM, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	m = updatedM.(MonitorDashboardModel)
	assert.NotNil(t, cmd)

	// Execute cmd
	msg = cmd()
	assert.True(t, resumeCalled)
	assert.Equal(t, "session2", resumeName)
	assert.Contains(t, msg.(actionResultMsg).msg, "Resumed session session2")

	// Test Kill (k) - Should enter confirmation mode
	m.table.SetCursor(0)
	updatedM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = updatedM.(MonitorDashboardModel)

	assert.Equal(t, "confirm_kill", m.viewMode)
	assert.Equal(t, "session1", m.sessionToKill)

	// Test Confirm Kill (y)
	updatedM, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = updatedM.(MonitorDashboardModel)

	assert.Equal(t, "list", m.viewMode)
	assert.Equal(t, "", m.sessionToKill)
	assert.NotNil(t, cmd)

	msg = cmd()
	assert.True(t, stopCalled)
	assert.Equal(t, "session1", stopName)
	assert.Contains(t, msg.(actionResultMsg).msg, "Stopped session session1")

	// Reset stopCalled for next test
	stopCalled = false

	// Test Cancel Kill (n)
	m.viewMode = "confirm_kill" // Force mode
	m.sessionToKill = "session1"

	updatedM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = updatedM.(MonitorDashboardModel)

	assert.Equal(t, "list", m.viewMode)
	assert.Equal(t, "", m.sessionToKill)
	assert.False(t, stopCalled) // Should not call stop again (stopCalled was true from previous test, need reset or just check count? mock.CallCount is better but simple bool ok if we reset)
	// We didn't reset stopCalled, but since we didn't execute any command, it shouldn't change.

	// Test Logs (l)
	m.table.SetCursor(0)
	updatedM, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m = updatedM.(MonitorDashboardModel)
	assert.NotNil(t, cmd)

	// Execute cmd (fetches logs)
	msg = cmd() // returns func that returns string
	// Wait, code says: return m, func() tea.Msg { return func() tea.Msg { return logs }() }
	// Double wrapping?
	/*
		return m, func() tea.Msg {
			logs, err := m.callbacks.GetLogs(name)
			if err != nil { return actionResultMsg{err: err} }
			return func() tea.Msg { return logs }()
		}
	*/
	// The outer cmd returns a Msg which IS the string (because the inner func is immediately executed? No).
	// `func() tea.Msg { return logs }` is a function, not a Msg.
	// Wait, tea.Msg is interface{}. So the Msg IS the function?
	// `case string:` handles string msg.

	// Let's trace carefully:
	// Outer func executes, calls GetLogs. Returns `func() tea.Msg { return logs }`.
	// So `msg` is `func() tea.Msg`.
	// Bubble Tea loop receives this Msg.
	// The Update loop switch msg.(type) ...
	// Does it handle `func() tea.Msg`? No.
	// This looks like a bug in `monitor_dashboard.go` or I misunderstood how BubbleTea handles func as Msg.
	// BubbleTea Cmd returns Msg.
	// If Msg is a function, BubbleTea does NOT execute it automatically unless it's a Cmd.
	// But Cmd IS `func() tea.Msg`.
	// So `cmd` returns `Msg`.
	// The `Msg` returned is `func() tea.Msg { return logs }`.
	// Bubble Tea `Update` receives `msg`.
	// `monitor_dashboard.go` handles:
	/*
	case string:
		m.logContent = msg
	*/
	// It expects `string`.
	// But the Cmd returns a function.

	// In `monitor_dashboard.go`:
	/*
		return func() tea.Msg { return logs }()
	*/
	// Ah! It calls the function immediately `...()`.
	// So it returns `logs` (string) as `tea.Msg`.
	// So `msg` IS `string`.

	logMsg := msg
	assert.Equal(t, "logs for session1", logMsg)

	// Now update with the log msg
	updatedM, _ = m.Update(logMsg)
	m = updatedM.(MonitorDashboardModel)
	assert.Equal(t, "logs", m.viewMode)
	assert.Equal(t, "logs for session1", m.logContent)

	// Test Quit Logs (q)
	updatedM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m = updatedM.(MonitorDashboardModel)
	assert.Equal(t, "list", m.viewMode)
}

func TestMonitorDashboardModel_Update_Error(t *testing.T) {
	callbacks := ActionCallbacks{
		Stop: func(name string) error { return errors.New("stop error") },
	}
	m := NewMonitorDashboardModel(callbacks)
	m.sessions = []model.UnifiedSession{{Name: "s1"}}
	m.sessionToKill = "s1"
	m.viewMode = "confirm_kill"

	updatedM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = updatedM.(MonitorDashboardModel)

	msg := cmd()
	assert.IsType(t, actionResultMsg{}, msg)
	assert.Equal(t, "stop error", msg.(actionResultMsg).err.Error())

	updatedM, cmd = m.Update(msg)
	m = updatedM.(MonitorDashboardModel)
	assert.Contains(t, m.message, "Error: stop error")
}

func TestMonitorDashboardModel_View(t *testing.T) {
	m := NewMonitorDashboardModel(ActionCallbacks{})
	m.sessions = []model.UnifiedSession{
		{Name: "s1", Status: "running", Location: "local", Cost: 0, Goal: "g"},
	}
	m.updateTableRows()

	// Setup window size
	updatedM, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updatedM.(MonitorDashboardModel)

	view := m.View()
	assert.Contains(t, view, "RECAC Control Center")
	assert.Contains(t, view, "s1")
	assert.Contains(t, view, "running")

	// Logs View
	m.viewMode = "logs"
	m.logContent = "Log Content"
	m.viewport.SetContent(m.logContent) // Viewport needs to be updated manually in this test setup
	view = m.View()
	assert.Contains(t, view, "Session Logs")
	assert.Contains(t, view, "Log Content")

	// Confirm Kill View
	m.viewMode = "confirm_kill"
	m.sessionToKill = "s1"
	view = m.View()
	assert.Contains(t, view, "DANGER ZONE")
	assert.Contains(t, view, "s1")
}
