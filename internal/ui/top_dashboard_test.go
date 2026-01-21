package ui

import (
	"errors"
	"recac/internal/model"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestTopDashboardModel_Update(t *testing.T) {
	m := NewTopDashboardModel()

	// Mock GetTopSessions
	mockSessions := []model.UnifiedSession{
		{
			Name:      "session-1",
			Status:    "Running",
			CPU:       "10%",
			Memory:    "100MB",
			StartTime: time.Now(),
			Goal:      "Goal 1",
		},
	}

	GetTopSessions = func() ([]model.UnifiedSession, error) {
		return mockSessions, nil
	}

	// Test Init
	cmd := m.Init()
	if cmd == nil {
		t.Error("Init returned nil cmd")
	}

	// Test Tick
	tickMsg := topTickMsg(time.Now())
	updatedModel, cmd := m.Update(tickMsg)
	m = updatedModel.(topDashboardModel)
	if cmd == nil {
		t.Error("Update(Tick) should return a command")
	}

	// Test Refreshed Msg
	refreshedMsg := topSessionsRefreshedMsg(mockSessions)
	updatedModel, _ = m.Update(refreshedMsg)
	m = updatedModel.(topDashboardModel)

	if len(m.sessions) != 1 {
		t.Errorf("Expected 1 session, got %d", len(m.sessions))
	}
	if m.sessions[0].Name != "session-1" {
		t.Errorf("Expected session name session-1, got %s", m.sessions[0].Name)
	}

	// Test View
	view := m.View()
	if view == "" {
		t.Error("View returned empty string")
	}

	// Test Window Resize
	resizeMsg := tea.WindowSizeMsg{Width: 100, Height: 50}
	updatedModel, _ = m.Update(resizeMsg)
	m = updatedModel.(topDashboardModel)
	if m.width != 100 || m.height != 50 {
		t.Error("Window size not updated")
	}
}

func TestTopDashboardModel_Error(t *testing.T) {
	m := NewTopDashboardModel()

	err := errors.New("fetch error")
	GetTopSessions = func() ([]model.UnifiedSession, error) {
		return nil, err
	}

	// Trigger error
	msgCmd := refreshTopSessionsCmd()
	msg := msgCmd()
	errMsg, ok := msg.(error)
	if !ok {
		t.Fatalf("Expected error msg, got %T", msg)
	}

	updatedModel, _ := m.Update(errMsg)
	m = updatedModel.(topDashboardModel)

	if m.err != err {
		t.Error("Model error state not updated")
	}

	if view := m.View(); view != "Error: fetch error" {
		t.Errorf("Expected error view, got %q", view)
	}
}

func TestStartTopDashboard(t *testing.T) {
	// StartTopDashboard runs the program which is interactive.
	// We can't easily test the full run, but we can verify it's a function that exists.
	if StartTopDashboard == nil {
		t.Error("StartTopDashboard is nil")
	}
}
