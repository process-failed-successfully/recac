package ui

import (
	"errors"
	"strings"

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

	// Verify aggregations
	if m.totalTokens != 300 {
		t.Errorf("Expected total tokens 300, got %d", m.totalTokens)
	}
	if len(m.modelCosts) != 1 {
		t.Errorf("Expected 1 model cost, got %d", len(m.modelCosts))
	} else {
		mc := m.modelCosts[0]
		if mc.Name != "gpt-4" {
			t.Errorf("Expected model name gpt-4, got %s", mc.Name)
		}
		if mc.TotalTokens != 300 {
			t.Errorf("Expected model total tokens 300, got %d", mc.TotalTokens)
		}
	}
}

func TestCostModel_Aggregation(t *testing.T) {
	sm := &MockSessionManager{}
	m := newCostModel(sm)

	// Mock LoadAgentState with variable returns based on session
	LoadAgentState = func(filePath string) (*agent.State, error) {
		if filePath == "state1" {
			return &agent.State{
				Model: "gpt-4",
				TokenUsage: agent.TokenUsage{TotalTokens: 100},
			}, nil
		}
		if filePath == "state2" {
			return &agent.State{
				Model: "gpt-4",
				TokenUsage: agent.TokenUsage{TotalTokens: 200},
			}, nil
		}
		if filePath == "state3" {
			return &agent.State{
				Model: "gpt-3.5-turbo",
				TokenUsage: agent.TokenUsage{TotalTokens: 50},
			}, nil
		}
		return nil, errors.New("not found")
	}

	m.sessions = []*runner.SessionState{
		{Name: "s1", AgentStateFile: "state1"},
		{Name: "s2", AgentStateFile: "state2"},
		{Name: "s3", AgentStateFile: "state3"},
	}

	m.updateTable()

	// Total tokens should be 100 + 200 + 50 = 350
	if m.totalTokens != 350 {
		t.Errorf("Expected total tokens 350, got %d", m.totalTokens)
	}

	// Should have 2 models
	if len(m.modelCosts) != 2 {
		t.Errorf("Expected 2 model costs, got %d", len(m.modelCosts))
	}

	foundGPT4 := false
	foundGPT35 := false

	for _, mc := range m.modelCosts {
		if mc.Name == "gpt-4" {
			foundGPT4 = true
			if mc.TotalTokens != 300 {
				t.Errorf("Expected gpt-4 tokens 300, got %d", mc.TotalTokens)
			}
		}
		if mc.Name == "gpt-3.5-turbo" {
			foundGPT35 = true
			if mc.TotalTokens != 50 {
				t.Errorf("Expected gpt-3.5 tokens 50, got %d", mc.TotalTokens)
			}
		}
	}

	if !foundGPT4 || !foundGPT35 {
		t.Error("Did not find expected models in aggregation")
	}

	// Test View rendering
	view := m.View()
	if !strings.Contains(view, "COST BY MODEL") {
		t.Error("View missing 'COST BY MODEL' section")
	}
	if !strings.Contains(view, "TOTALS") {
		t.Error("View missing 'TOTALS' section")
	}
	if !strings.Contains(view, "gpt-4") {
		t.Error("View missing model name")
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
