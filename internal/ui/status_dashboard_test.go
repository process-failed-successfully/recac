package ui

import (
	"errors"
	"recac/internal/agent"
	"recac/internal/runner"
	"testing"
	"time"
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

	GetSessionStatus = func(name string) (*runner.SessionState, *agent.State, error) {
		if name != sessionName {
			return nil, nil, errors.New("wrong session name")
		}
		return mockSession, mockAgentState, nil
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

	// Test View
	view := m.View()
	if view == "" {
		t.Error("View returned empty string")
	}
}

func TestStatusDashboardModel_Error(t *testing.T) {
	m := NewStatusDashboardModel("test-session")

	err := errors.New("fetch error")
	GetSessionStatus = func(name string) (*runner.SessionState, *agent.State, error) {
		return nil, nil, err
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
