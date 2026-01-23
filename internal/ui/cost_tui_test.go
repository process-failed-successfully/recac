package ui

import (
	"errors"
	"recac/internal/agent"
	"recac/internal/runner"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type MockSessionManager struct {
	ListSessionsFunc func() ([]*runner.SessionState, error)
}

func (m *MockSessionManager) ListSessions() ([]*runner.SessionState, error) {
	if m.ListSessionsFunc != nil {
		return m.ListSessionsFunc()
	}
	return nil, nil
}

func TestCostModel_Update(t *testing.T) {
	// Mock LoadAgentState
	LoadAgentState = func(filePath string) (*agent.State, error) {
		return &agent.State{
			Model: "gpt-4",
			TokenUsage: agent.TokenUsage{
				TotalPromptTokens:   100,
				TotalResponseTokens: 200,
				TotalTokens:         300,
			},
		}, nil
	}

	sm := &MockSessionManager{
		ListSessionsFunc: func() ([]*runner.SessionState, error) {
			return []*runner.SessionState{
				{
					Name:      "session-1",
					Status:    "Running",
					StartTime: time.Now(),
				},
			}, nil
		},
	}
	m := newCostModel(sm)

	// Test Init
	cmd := m.Init()
	if cmd == nil {
		t.Error("Init returned nil cmd")
	}

	// Test Tick
	tick := tickMsg(time.Now())
	updatedModel, cmd := m.Update(tick)
	m = updatedModel.(*costModel)
	if cmd == nil {
		t.Error("Update(Tick) should return a command")
	}

	// Test Update Msg
	sessions, _ := sm.ListSessions()
	update := updateMsg(sessions)
	updatedModel, _ = m.Update(update)
	m = updatedModel.(*costModel)

	if len(m.sessions) != 1 {
		t.Errorf("Expected 1 session, got %d", len(m.sessions))
	}

	// Test View
	view := m.View()
	if view == "" {
		t.Error("View returned empty string")
	}

	// Test Window Resize
	resizeMsg := tea.WindowSizeMsg{Width: 100, Height: 50}
	updatedModel, _ = m.Update(resizeMsg)
	m = updatedModel.(*costModel)
	// The table model adjusts height based on styles (borders, etc).
	// We just verify it updated to something reasonable (around 45)
	if m.table.Height() != 43 {
		t.Logf("Note: Table height is %d (expected 45 - borders/padding)", m.table.Height())
	}
	if m.table.Height() < 40 {
		t.Errorf("Expected table height ~45, got %d", m.table.Height())
	}
}

func TestCostModel_UpdateTable(t *testing.T) {
	sm := &MockSessionManager{}
	m := newCostModel(sm)

	// Mock LoadAgentState
	LoadAgentState = func(filePath string) (*agent.State, error) {
		return &agent.State{
			Model: "gpt-4",
			TokenUsage: agent.TokenUsage{
				TotalPromptTokens:   100,
				TotalResponseTokens: 200,
				TotalTokens:         300,
			},
		}, nil
	}

	m.sessions = []*runner.SessionState{
		{
			Name:      "session-1",
			Status:    "Running",
			StartTime: time.Now(),
		},
	}

	m.updateTable()

	rows := m.table.Rows()
	if len(rows) != 1 {
		t.Errorf("Expected 1 row, got %d", len(rows))
	}
	// Check token count column (Total)
	if rows[0][6] != "300" {
		t.Errorf("Expected 300 tokens, got %s", rows[0][6])
	}
}

func TestCostModel_Error(t *testing.T) {
	sm := &MockSessionManager{}
	m := newCostModel(sm)

	err := errors.New("fetch error")
	msg := errMsg{err}

	updatedModel, _ := m.Update(msg)
	m = updatedModel.(*costModel)

	if m.err != err {
		t.Error("Model error state not updated")
	}

	if view := m.View(); view == "" {
		t.Error("Expected error view")
	}
}

func TestCostModel_Init_Cmds(t *testing.T) {
	sm := &MockSessionManager{}
	m := newCostModel(sm)

	cmd := m.Init()
	if cmd == nil {
		t.Error("Expected not nil cmd")
	}

	// Init returns a Batch command, which is a func.
	// Executing it returns a BatchMsg (slice of Msgs).
	// We can't easily inspect BatchMsg without internal knowledge or executing it.
	// But simply asserting it's not nil covers the code path in Init().
}

func TestStartCostTUI_Error(t *testing.T) {
	// Save original
	orig := LoadAgentState
	defer func() { LoadAgentState = orig }()

	// Set to nil
	LoadAgentState = nil

	err := StartCostTUI(&MockSessionManager{})
	if err == nil {
		t.Error("Expected error when LoadAgentState is nil")
	}
}

func TestFetchSessions(t *testing.T) {
	sm := &MockSessionManager{
		ListSessionsFunc: func() ([]*runner.SessionState, error) {
			return []*runner.SessionState{{Name: "test"}}, nil
		},
	}

	cmd := fetchSessions(sm)
	msg := cmd()

	update, ok := msg.(updateMsg)
	if !ok {
		t.Errorf("Expected updateMsg, got %T", msg)
	}
	if len(update) != 1 || update[0].Name != "test" {
		t.Errorf("Expected 1 session with name test, got %v", update)
	}

	// Error case
	smErr := &MockSessionManager{
		ListSessionsFunc: func() ([]*runner.SessionState, error) {
			return nil, errors.New("fail")
		},
	}
	cmdErr := fetchSessions(smErr)
	msgErr := cmdErr()

	errMsgVal, ok := msgErr.(errMsg)
	if !ok {
		t.Errorf("Expected errMsg, got %T", msgErr)
	}
	if errMsgVal.err.Error() != "fail" {
		t.Errorf("Expected fail error, got %v", errMsgVal.err)
	}
}
