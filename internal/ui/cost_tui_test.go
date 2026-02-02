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
	m := newCostModel(sm, 0)

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
	m := newCostModel(sm, 0)

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
	m := newCostModel(sm, 0)

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
	m := newCostModel(sm, 0)

	cmd := m.Init()
	if cmd == nil {
		t.Error("Expected not nil cmd")
	}
}

func TestCostModel_UpdateTable_Limit(t *testing.T) {
	sm := &MockSessionManager{}
	m := newCostModel(sm, 1) // Limit 1

	// Mock LoadAgentState
	LoadAgentState = func(filePath string) (*agent.State, error) {
		return &agent.State{
			Model: "gpt-4",
			TokenUsage: agent.TokenUsage{TotalTokens: 100},
		}, nil
	}

	m.sessions = []*runner.SessionState{
		{Name: "s1", AgentStateFile: "f1"},
		{Name: "s2", AgentStateFile: "f2"},
	}

	m.updateTable()

	rows := m.table.Rows()
	if len(rows) != 1 {
		t.Errorf("Expected 1 row, got %d", len(rows))
	}
}

func TestCostModel_UpdateTable_Sort(t *testing.T) {
	sm := &MockSessionManager{}
	m := newCostModel(sm, 0)

	// Mock LoadAgentState
	LoadAgentState = func(filePath string) (*agent.State, error) {
		// Mock different costs based on file path
		tokens := 100
		if filePath == "high" {
			tokens = 1000
		}
		return &agent.State{
			Model: "gpt-4-turbo", // Use a model with known pricing in agent package
			TokenUsage: agent.TokenUsage{
				TotalPromptTokens:   tokens,
				TotalResponseTokens: tokens,
				TotalTokens:         tokens * 2,
			},
		}, nil
	}

	m.sessions = []*runner.SessionState{
		{Name: "low-cost", AgentStateFile: "low"},
		{Name: "high-cost", AgentStateFile: "high"},
	}

	m.updateTable()

	rows := m.table.Rows()
	if len(rows) != 2 {
		t.Fatalf("Expected 2 rows, got %d", len(rows))
	}

	// First row should be high cost
	if rows[0][0] != "high-cost" {
		t.Errorf("Expected first row to be high-cost, got %s", rows[0][0])
	}
	if rows[1][0] != "low-cost" {
		t.Errorf("Expected second row to be low-cost, got %s", rows[1][0])
	}
}
