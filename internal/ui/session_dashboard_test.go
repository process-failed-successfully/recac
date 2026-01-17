package ui

import (
	"recac/internal/agent"
	"recac/internal/model"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSessionDashboardModel_Update(t *testing.T) {
	// Setup Mocks
	mockSession := &model.UnifiedSession{
		Name:      "test-session",
		Status:    "running",
		Goal:      "Fix bugs",
		StartTime: time.Now(),
		CPU:       "10%",
		Memory:    "100MB",
	}

	mockState := &agent.State{
		Model: "gpt-4",
		History: []agent.Message{
			{Role: "user", Content: "Do this"},
			{Role: "assistant", Content: "Thinking..."},
		},
	}

	mockLogs := "Log line 1\nLog line 2"

	// Inject Mocks
	GetSessionDetail = func(name string) (*model.UnifiedSession, error) {
		return mockSession, nil
	}
	GetAgentState = func(name string) (*agent.State, error) {
		return mockState, nil
	}
	GetSessionLogs = func(name string) (string, error) {
		return mockLogs, nil
	}

	m := NewSessionDashboardModel("test-session")

	// Trigger Window Resize
	// We need to cast back to SessionDashboardModel after updateModel because updateModel returns tea.Model interface
	res, _ := updateModel(m, tea.WindowSizeMsg{Width: 100, Height: 50})
	m = res.(SessionDashboardModel)

	// Trigger Data Refresh
	cmd := refreshSessionDataCmd("test-session")
	msg := cmd()
	res, _ = updateModel(m, msg)
	m = res.(SessionDashboardModel)

	// Assertions
	// m is already SessionDashboardModel struct, not interface
	if m.session == nil || m.session.Name != "test-session" {
		t.Errorf("Expected session name 'test-session', got %v", m.session)
	}
	if m.logs != mockLogs {
		t.Errorf("Expected logs match")
	}
	if len(m.agentState.History) != 2 {
		t.Errorf("Expected 2 history items")
	}

	// View rendering check
	view := m.View()
	if !strings.Contains(view, "test-session") {
		t.Errorf("View should contain session name")
	}
	if !strings.Contains(view, "Log line 1") {
		t.Errorf("View should contain logs")
	}
	if !strings.Contains(view, "Thinking...") {
		t.Errorf("View should contain agent thought")
	}
}

// Helper to cast and update
func updateModel(m tea.Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	return m.Update(msg)
}
