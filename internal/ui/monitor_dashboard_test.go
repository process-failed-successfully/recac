package ui

import (
	"errors"
	"recac/internal/model"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewMonitorDashboardModel(t *testing.T) {
	cb := ActionCallbacks{}
	m := NewMonitorDashboardModel(cb)

	if m.viewMode != "list" {
		t.Errorf("Expected viewMode list, got %s", m.viewMode)
	}
}

func TestMonitorDashboardModel_Init(t *testing.T) {
	m := NewMonitorDashboardModel(ActionCallbacks{
		GetSessions: func() ([]model.UnifiedSession, error) { return nil, nil },
	})
	cmd := m.Init()
	if cmd == nil {
		t.Error("Init should return batch cmd")
	}
}

func TestMonitorDashboardModel_Update(t *testing.T) {
	var cmd tea.Cmd
	cb := ActionCallbacks{
		GetSessions: func() ([]model.UnifiedSession, error) {
			return []model.UnifiedSession{{Name: "test-session", Status: "running"}}, nil
		},
		Stop: func(name string) error {
			if name == "fail" {
				return errors.New("stop error")
			}
			return nil
		},
		Pause: func(name string) error { return nil },
		Resume: func(name string) error { return nil },
		GetLogs: func(name string) (string, error) { return "logs", nil },
	}
	m := NewMonitorDashboardModel(cb)

	// 1. Refresh Sessions
	sessions := []model.UnifiedSession{{Name: "test-session", Status: "running"}}
	m, _ = updateMonitor(m, monitorSessionsRefreshedMsg(sessions))

	if len(m.sessions) != 1 {
		t.Errorf("Expected 1 session, got %d", len(m.sessions))
	}
	if len(m.table.Rows()) == 0 || m.table.Rows()[0][0] != "test-session" {
		t.Error("Table not updated")
	}

	// 2. Window Resize
	m, _ = updateMonitor(m, tea.WindowSizeMsg{Width: 100, Height: 50})
	if m.width != 100 {
		t.Error("Resize failed")
	}

	// 3. Selection and Kill
	m.table.SetCursor(0)
	// Press 'k'
	m, _ = updateMonitor(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.viewMode != "confirm_kill" {
		t.Error("Expected confirm_kill mode")
	}
	if m.sessionToKill != "test-session" {
		t.Error("Expected sessionToKill to be set")
	}

	// Cancel Kill
	m, _ = updateMonitor(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if m.viewMode != "list" {
		t.Error("Expected list mode after cancel")
	}

	// Confirm Kill
	m.table.SetCursor(0) // Ensure cursor is still correct
	m, _ = updateMonitor(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m, cmd = updateMonitor(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})

	// Run the command
	msg := cmd()
	if res, ok := msg.(actionResultMsg); ok {
		if res.err != nil {
			t.Error("Stop returned error")
		}
		if !strings.Contains(res.msg, "Stopped") {
			t.Error("Expected success message")
		}
	} else {
		t.Errorf("Expected actionResultMsg, got %T", msg)
	}
	// Process result msg
	if msg != nil {
		m, _ = updateMonitor(m, msg)
		if m.message == "" {
			t.Error("Expected status message set")
		}
	}

	// 4. Logs
	m.table.SetCursor(0)
	m, cmd = updateMonitor(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})

	if cmd == nil {
		t.Fatal("Expected log command")
	}

	// Run log command
	logMsg := cmd()

	if str, ok := logMsg.(string); ok {
		if str != "logs" {
			t.Error("Expected 'logs'")
		}

		// Update with log string
		m, _ = updateMonitor(m, str)
		if m.viewMode != "logs" {
			t.Error("Expected logs mode")
		}
		if !strings.Contains(m.viewport.View(), "logs") {
			t.Error("Viewport should show logs")
		}
	} else {
		// Maybe it returned actionResultMsg if error?
		t.Errorf("Expected string log msg, got %T", logMsg)
	}

	// Quit logs
	m, _ = updateMonitor(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if m.viewMode != "list" {
		t.Error("Expected list mode")
	}

    // 5. Pause/Resume
    m.table.SetCursor(0)
    m, cmd = updateMonitor(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
    if cmd != nil {
	msg = cmd()
	if res, ok := msg.(actionResultMsg); ok {
	        if !strings.Contains(res.msg, "Paused") {
	            t.Error("Expected Paused")
	        }
	    }
    }
}

func TestMonitorDashboardModel_View(t *testing.T) {
	m := NewMonitorDashboardModel(ActionCallbacks{})
	m.sessions = []model.UnifiedSession{{Name: "test"}}
	m.updateTableRows() // Ensure table populated

	view := m.View()
	if !strings.Contains(view, "Control Center") {
		t.Error("View should contain title")
	}
	if !strings.Contains(view, "test") {
		t.Error("View should contain session name")
	}

	m.viewMode = "logs"
	view = m.View()
	if !strings.Contains(view, "Session Logs") {
		t.Error("Log view should contain title")
	}

	m.viewMode = "confirm_kill"
	m.sessionToKill = "sess"
	view = m.View()
	if !strings.Contains(view, "Are you sure") {
		t.Error("Confirm view should contain prompt")
	}
}

// Helper
func updateMonitor(m MonitorDashboardModel, msg tea.Msg) (MonitorDashboardModel, tea.Cmd) {
	mod, cmd := m.Update(msg)
	return mod.(MonitorDashboardModel), cmd
}
