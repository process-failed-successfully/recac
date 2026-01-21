package ui

import (
	"fmt"
	"recac/internal/agent"
	"recac/internal/runner"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

type mockCostSessionManager struct {
	sessions []*runner.SessionState
	err      error
}

func (m *mockCostSessionManager) ListSessions() ([]*runner.SessionState, error) {
	return m.sessions, m.err
}

func TestCostModel_Init(t *testing.T) {
	sm := &mockCostSessionManager{}
	m := newCostModel(sm)
	cmd := m.Init()
	assert.NotNil(t, cmd)
}

func TestCostModel_Update(t *testing.T) {
	sm := &mockCostSessionManager{}
	m := newCostModel(sm)

	t.Run("quit message", func(t *testing.T) {
		newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
		assert.NotNil(t, cmd)
		_, ok := newM.(*costModel)
		assert.True(t, ok)
	})

	t.Run("window size", func(t *testing.T) {
		newM, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
		_, ok := newM.(*costModel)
		assert.True(t, ok)
	})

	t.Run("successful session fetch", func(t *testing.T) {
		// Mock LoadAgentState
		originalLoad := LoadAgentState
		defer func() { LoadAgentState = originalLoad }()
		LoadAgentState = func(filePath string) (*agent.State, error) {
			return &agent.State{
				TokenUsage: agent.TokenUsage{TotalTokens: 100},
				Model:      "gpt-4",
			}, nil
		}

		sessions := []*runner.SessionState{
			{Name: "s1", Status: "running", StartTime: time.Now()},
		}
		newM, _ := m.Update(updateMsg(sessions))
		updatedM := newM.(*costModel)
		assert.Len(t, updatedM.sessions, 1)
	})

	t.Run("error message", func(t *testing.T) {
		err := fmt.Errorf("fetch error")
		newM, _ := m.Update(errMsg{err})
		updatedM := newM.(*costModel)
		assert.Equal(t, err, updatedM.err)
	})
}

func TestCostModel_View(t *testing.T) {
	sm := &mockCostSessionManager{}
	m := newCostModel(sm)

	// Error view
	m.err = fmt.Errorf("some error")
	assert.Contains(t, m.View(), "Error: some error")

	// Normal view
	m.err = nil
	assert.Contains(t, m.View(), "RECAC Live Session Monitor")
}

func TestFetchSessions(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		sm := &mockCostSessionManager{
			sessions: []*runner.SessionState{{Name: "s1"}},
		}
		cmd := fetchSessions(sm)
		msg := cmd()
		res, ok := msg.(updateMsg)
		assert.True(t, ok)
		assert.Len(t, res, 1)
	})

	t.Run("error", func(t *testing.T) {
		sm := &mockCostSessionManager{
			err: fmt.Errorf("fail"),
		}
		cmd := fetchSessions(sm)
		msg := cmd()
		res, ok := msg.(errMsg)
		assert.True(t, ok)
		assert.EqualError(t, res.err, "fail")
	})
}
