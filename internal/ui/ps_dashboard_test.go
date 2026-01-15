package ui

import (
	"strings"
	"testing"
	"time"

	"recac/internal/model"
)

func TestPsDashboardView(t *testing.T) {
	// Mock the GetSessions function
	GetSessions = func() ([]model.UnifiedSession, error) {
		return []model.UnifiedSession{
			{Name: "session-1", Status: "running", Location: "local", LastActivity: time.Now(), Goal: "test goal"},
		}, nil
	}
	defer func() { GetSessions = nil }()

	m := NewPsDashboardModel()

	// Simulate one update to populate rows
	m.sessions = []model.UnifiedSession{
		{Name: "session-1", Status: "running", Location: "local", LastActivity: time.Now(), Goal: "test goal"},
	}
	m.updateTableRows()

	view := m.View()

	// Check that the old hardcoded help is GONE
	if strings.Contains(view, "(press 'q' to quit)") {
		t.Errorf("View still contains old hardcoded help text")
	}

	// Check for new help text
	// bubbles/help usually renders the keys defined in ShortHelp
	// Our keys are "↑/k", "↓/j", "q"
	if !strings.Contains(view, "↑/k") {
		t.Errorf("View missing help for Up key")
	}
	if !strings.Contains(view, "q quit") {
		t.Errorf("View missing help for Quit key")
	}
}
