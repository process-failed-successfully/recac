package ui

import (
	"errors"
	"recac/internal/agent"
	"recac/internal/runner"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestStatusDashboardModel_Update(t *testing.T) {
	sessionName := "test-session"
	m := NewStatusDashboardModel(sessionName)

	// Mock GetSessionStatus
	mockSession := &runner.SessionState{
		Name:      sessionName,
		Status:    "running",
		Goal:      "Test Goal",
		StartTime: time.Now(),
	}
	mockAgentState := &agent.State{
		Model: "gpt-4",
		TokenUsage: agent.TokenUsage{
			TotalTokens: 100,
		},
	}

	GetSessionStatus = func(name string) (*runner.SessionState, *agent.State, string, error) {
		if name != sessionName {
			return nil, nil, "", errors.New("wrong session name")
		}
		return mockSession, mockAgentState, "1 file changed", nil
	}

	// Test Init
	cmd := m.Init()
	if cmd == nil {
		t.Error("Init returned nil cmd")
	}

	// Test Tick (should trigger refresh)
	tickMsg := statusTickMsg(time.Now())
	updatedModel, cmd := m.Update(tickMsg)
	m = updatedModel.(statusDashboardModel)

	// Verify that a batch command is returned (containing refresh and next tick)
	if cmd == nil {
		t.Error("Update(Tick) should return a command")
	}

	// Because we are using tea.Batch, we can't easily inspect the internal commands without executing them.
	// However, we can test that the model update logic for the RefreshedMsg still works.
	// We can manually execute refreshStatusCmd to get the message we expect.

	msgCmd := refreshStatusCmd(sessionName)
	msg := msgCmd()
	refreshedMsg, ok := msg.(statusRefreshedMsg)
	if !ok {
		t.Fatalf("Expected statusRefreshedMsg from direct call, got %T", msg)
	}

	if refreshedMsg.session.Name != sessionName {
		t.Errorf("Expected session name %s, got %s", sessionName, refreshedMsg.session.Name)
	}

	// Test Update with Refreshed Msg
	updatedModel, _ = m.Update(refreshedMsg)
	m = updatedModel.(statusDashboardModel)

	if m.session == nil || m.session.Name != sessionName {
		t.Error("Model session not updated")
	}
	if m.agentState == nil || m.agentState.Model != "gpt-4" {
		t.Error("Model agentState not updated")
	}

	if m.gitDiffStat != "1 file changed" {
		t.Errorf("Expected git diff stat '1 file changed', got '%s'", m.gitDiffStat)
	}

	// Test View
	view := m.View()
	if view == "" {
		t.Error("View returned empty string")
	}
}

func TestStatusDashboardModel_Error(t *testing.T) {
	m := NewStatusDashboardModel("test-session")

	err := errors.New("fetch error")
	GetSessionStatus = func(name string) (*runner.SessionState, *agent.State, string, error) {
		return nil, nil, "", err
	}

	// Trigger update via command execution simulation
	cmd := refreshStatusCmd("test-session")
	msg := cmd()

	// msg should be error
	errMsg, ok := msg.(error)
	if !ok {
		t.Fatalf("Expected error msg, got %T", msg)
	}

	updatedModel, _ := m.Update(errMsg)
	m = updatedModel.(statusDashboardModel)

	if m.err != err {
		t.Error("Model error state not updated")
	}

	if view := m.View(); view != "Error: fetch error" {
		t.Errorf("Expected error view, got %q", view)
	}
}

func TestStatusDashboardModel_Update_WindowSize(t *testing.T) {
	m := NewStatusDashboardModel("test-session")
	updatedModel, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	m = updatedModel.(statusDashboardModel)

	if m.width != 100 {
		t.Errorf("Expected width 100, got %d", m.width)
	}
	if m.height != 50 {
		t.Errorf("Expected height 50, got %d", m.height)
	}
}

func TestStatusDashboardModel_Update_Keys(t *testing.T) {
	m := NewStatusDashboardModel("test-session")

	// Test Quit keys
	keys := []string{"q", "ctrl+c"}
	for _, key := range keys {
		updatedModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key), Alt: false})
		if key == "ctrl+c" {
			updatedModel, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
		}

		if cmd == nil {
			t.Errorf("Expected quit command for key %s", key)
		} else {
			// tea.Quit is a special command, harder to assert exact equality without executing
			// but returning a non-nil cmd for these keys usually implies Quit in this simple model
		}
		_ = updatedModel
	}

	// Test ignored key
	updatedModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if cmd != nil {
		t.Error("Expected nil command for ignored key")
	}
	_ = updatedModel
}

func TestStatusDashboardModel_RenderLastActivity_WithHistory(t *testing.T) {
	state := &agent.State{
		History: []agent.Message{
			{
				Role:      "user",
				Content:   "Hello",
				Timestamp: time.Now(),
			},
			{
				Role:      "agent",
				Content:   "Hi there\nHow can I help?",
				Timestamp: time.Now(),
			},
		},
	}

	output := renderLastActivity(state, 80)
	if output == "" {
		t.Error("Expected output for history")
	}

	if !time.Now().After(state.History[1].Timestamp) {
		// Just ensuring timestamp logic didn't panic
	}
}
