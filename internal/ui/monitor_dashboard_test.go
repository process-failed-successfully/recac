package ui

import (
	"fmt"
	"recac/internal/model"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewMonitorDashboardModel(t *testing.T) {
	callbacks := ActionCallbacks{
		GetSessions: func() ([]model.UnifiedSession, error) {
			return nil, nil
		},
	}
	m := NewMonitorDashboardModel(callbacks)
	if m.viewMode != "list" {
		t.Errorf("expected viewMode to be 'list', got %s", m.viewMode)
	}
}

func TestMonitorDashboardModel_Init(t *testing.T) {
	m := NewMonitorDashboardModel(ActionCallbacks{})
	cmd := m.Init()
	if cmd == nil {
		t.Error("Init should return a command")
	}
	// Verify it's a batch command by running it and checking result type if possible,
	// or just ensuring it's not nil. BubbleTea cmds are opaque functions.
}

func TestMonitorDashboardModel_Update(t *testing.T) {
	// Setup mock sessions
	mockSessions := []model.UnifiedSession{
		{Name: "session1", Status: "running", HasCost: true, Cost: 1.23, Goal: "Test Goal"},
	}

	callbacks := ActionCallbacks{
		GetSessions: func() ([]model.UnifiedSession, error) {
			return mockSessions, nil
		},
		Stop: func(name string) error {
			if name == "session1" {
				return nil
			}
			return fmt.Errorf("unknown session")
		},
		GetLogs: func(name string) (string, error) {
			if name == "session1" {
				return "logs", nil
			}
			return "", fmt.Errorf("no logs")
		},
		Pause: func(name string) error {
			if name == "session1" {
				return nil
			}
			return fmt.Errorf("cannot pause")
		},
		Resume: func(name string) error {
			return nil
		},
	}

	m := NewMonitorDashboardModel(callbacks)
	m.width = 100
	m.height = 50
	m.table.SetWidth(100)
	m.table.SetHeight(40)

	// Simulate initial load
	m.sessions = mockSessions
	m.updateTableRows()

	// 1. Test WindowSizeMsg
	width, height := 120, 60
	updatedModel, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	m = updatedModel.(MonitorDashboardModel)
	if m.width != width || m.height != height {
		t.Errorf("WindowSizeMsg failed: expected %dx%d, got %dx%d", width, height, m.width, m.height)
	}

	// 2. Test monitorSessionsRefreshedMsg
	updatedModel, _ = m.Update(monitorSessionsRefreshedMsg(mockSessions))
	m = updatedModel.(MonitorDashboardModel)
	if len(m.sessions) != 1 {
		t.Errorf("monitorSessionsRefreshedMsg failed: expected 1 session, got %d", len(m.sessions))
	}
	if m.sessions[0].Name != "session1" {
		t.Errorf("monitorSessionsRefreshedMsg failed: expected session1, got %s", m.sessions[0].Name)
	}

	// 3. Test actionResultMsg (success)
	updatedModel, cmd := m.Update(actionResultMsg{msg: "Success"})
	m = updatedModel.(MonitorDashboardModel)
	if m.message != "Success" {
		t.Errorf("actionResultMsg failed: expected 'Success', got %s", m.message)
	}
	if cmd == nil {
		t.Error("Expected tick command to clear message")
	}

	// 4. Test actionResultMsg (error)
	updatedModel, _ = m.Update(actionResultMsg{err: fmt.Errorf("Fail")})
	m = updatedModel.(MonitorDashboardModel)
	if m.message != "Error: Fail" {
		t.Errorf("actionResultMsg failed: expected 'Error: Fail', got %s", m.message)
	}

	// 5. Test 'l' key (logs) - Simulate by sending key 'l'
	m.table.SetCursor(0)
	updatedModel, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	m = updatedModel.(MonitorDashboardModel)
	if cmd == nil {
		t.Error("Expected command for logs")
	} else {
		// Execute command to get logs
		msg := cmd()
		// It returns a function that returns a string (hacky implementation in source)
		if fn, ok := msg.(func() tea.Msg); ok {
			res := fn()
			if str, ok := res.(string); ok {
				if str != "logs" {
					t.Errorf("Expected logs 'logs', got %s", str)
				}
				// Now feed the string back to Update
				updatedModel, _ = m.Update(str)
				m = updatedModel.(MonitorDashboardModel)
				if m.viewMode != "logs" {
					t.Errorf("Expected viewMode 'logs', got %s", m.viewMode)
				}
			} else {
				t.Errorf("Expected string msg from logs cmd, got %T", res)
			}
		} else {
			// Or maybe it returns actionResultMsg if error?
			// The implementation: return func() tea.Msg { return logs }() which returns string directly?
			// Wait: return func() tea.Msg { return logs }()
			// No, it returns the result of calling that function.
			// So msg should be string "logs".
			if str, ok := msg.(string); ok {
				if str != "logs" {
					t.Errorf("Expected logs 'logs', got %s", str)
				}
				updatedModel, _ = m.Update(str)
				m = updatedModel.(MonitorDashboardModel)
				if m.viewMode != "logs" {
					t.Errorf("Expected viewMode 'logs', got %s", m.viewMode)
				}
			} else if _, ok := msg.(actionResultMsg); ok {
				// error case
			}
		}
	}

	// 6. Test 'q' in logs mode to return to list
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	m = updatedModel.(MonitorDashboardModel)
	if m.viewMode != "list" {
		t.Errorf("Expected viewMode 'list' after q, got %s", m.viewMode)
	}

	// 7. Test 'k' (kill)
	m.table.SetCursor(0)
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	m = updatedModel.(MonitorDashboardModel)
	if m.viewMode != "confirm_kill" {
		t.Errorf("Expected viewMode 'confirm_kill', got %s", m.viewMode)
	}
	if m.sessionToKill != "session1" {
		t.Errorf("Expected sessionToKill 'session1', got %s", m.sessionToKill)
	}

	// 8. Test 'y' in confirm_kill
	updatedModel, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	m = updatedModel.(MonitorDashboardModel) // Type assertion
	if m.viewMode != "list" {
		t.Errorf("Expected viewMode 'list' after y, got %s", m.viewMode)
	}

	// Execute the command returned by 'y'
	if cmd != nil {
		msg := cmd()
		// It returns a func that returns actionResultMsg
		if fn, ok := msg.(func() tea.Msg); ok {
			res := fn()
			if r, ok := res.(actionResultMsg); ok {
				if r.msg != "Stopped session session1" {
					t.Errorf("Unexpected message: %s", r.msg)
				}
			}
		}
	}

	// 9. Test 'n' in confirm_kill
	m.viewMode = "confirm_kill"
	m.sessionToKill = "session1"
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	m = updatedModel.(MonitorDashboardModel)
	if m.viewMode != "list" {
		t.Errorf("Expected viewMode 'list' after n, got %s", m.viewMode)
	}
	if m.sessionToKill != "" {
		t.Errorf("Expected sessionToKill empty, got %s", m.sessionToKill)
	}

	// 10. Test 'p' (pause/resume)
	m.table.SetCursor(0)
	// Pause running session
	updatedModel, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	m = updatedModel.(MonitorDashboardModel)
	if cmd != nil {
		msg := cmd()
		if fn, ok := msg.(func() tea.Msg); ok {
			res := fn()
			if r, ok := res.(actionResultMsg); ok {
				if r.msg != "Paused session session1" {
					t.Errorf("Unexpected message: %s", r.msg)
				}
			}
		}
	}

	// Resume paused session
	m.sessions[0].Status = "paused"
	m.updateTableRows()
	updatedModel, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	m = updatedModel.(MonitorDashboardModel)
	if cmd != nil {
		msg := cmd()
		if fn, ok := msg.(func() tea.Msg); ok {
			res := fn()
			if r, ok := res.(actionResultMsg); ok {
				if r.msg != "Resumed session session1" {
					t.Errorf("Unexpected message: %s", r.msg)
				}
			}
		}
	}

	// Test monitorTickMsg
	updatedModel, cmd = m.Update(monitorTickMsg(time.Now()))
	if cmd == nil {
		t.Error("Expected batch command from tick")
	}
}

func TestMonitorDashboardModel_View(t *testing.T) {
	m := NewMonitorDashboardModel(ActionCallbacks{})
	m.width = 100
	m.height = 50
	m.lastUpdate = time.Now()

	// List view empty
	view := m.View()
	if len(view) == 0 {
		t.Error("View returned empty string")
	}

	// List view with sessions
	m.sessions = []model.UnifiedSession{
		{Name: "sess1", Status: "running", HasCost: true, Cost: 10.5, Goal: "Goal"},
	}
	m.updateTableRows()
	view = m.View()
	if len(view) == 0 {
		t.Error("View returned empty string with sessions")
	}

	// Logs view
	m.viewMode = "logs"
	m.logContent = "some logs"
	view = m.View()
	if len(view) == 0 {
		t.Error("View returned empty string in logs mode")
	}

	// Confirm kill view
	m.viewMode = "confirm_kill"
	m.sessionToKill = "sess1"
	view = m.View()
	if len(view) == 0 {
		t.Error("View returned empty string in confirm_kill mode")
	}
}

func TestRefreshMonitorSessionsCmd(t *testing.T) {
	// Case 1: Success
	getSessions := func() ([]model.UnifiedSession, error) {
		return []model.UnifiedSession{{Name: "s1"}}, nil
	}
	cmd := refreshMonitorSessionsCmd(getSessions)
	msg := cmd()
	if sessions, ok := msg.(monitorSessionsRefreshedMsg); !ok {
		t.Errorf("Expected monitorSessionsRefreshedMsg, got %T", msg)
	} else if len(sessions) != 1 {
		t.Errorf("Expected 1 session, got %d", len(sessions))
	}

	// Case 2: Error
	getSessionsErr := func() ([]model.UnifiedSession, error) {
		return nil, fmt.Errorf("fail")
	}
	cmdErr := refreshMonitorSessionsCmd(getSessionsErr)
	msgErr := cmdErr()
	if res, ok := msgErr.(actionResultMsg); !ok {
		t.Errorf("Expected actionResultMsg, got %T", msgErr)
	} else if res.err == nil || res.err.Error() != "fail" {
		t.Errorf("Expected error 'fail', got %v", res.err)
	}

	// Case 3: Nil callback
	cmdNil := refreshMonitorSessionsCmd(nil)
	msgNil := cmdNil()
	if msgNil != nil {
		t.Errorf("Expected nil msg for nil callback, got %v", msgNil)
	}
}
