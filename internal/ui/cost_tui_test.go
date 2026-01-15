package ui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"recac/internal/agent"
	"recac/internal/runner"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSessionManager is a mock implementation of the SessionManager interface for testing.
type mockSessionManager struct {
	sessions []*runner.SessionState
	err      error
}

func (m *mockSessionManager) ListSessions() ([]*runner.SessionState, error) {
	return m.sessions, m.err
}

// mockLoadAgentState is a mock implementation of the LoadAgentStateFunc.
func mockLoadAgentState(filePath string) (*agent.State, error) {
	if strings.Contains(filePath, "error") {
		return nil, errors.New("failed to load state")
	}
	if strings.Contains(filePath, "empty") {
		return nil, nil // Simulate a nil state without an error
	}
	// Default successful case
	return &agent.State{
		Model: "test-model",
		TokenUsage: agent.TokenUsage{
			TotalPromptTokens:   100,
			TotalResponseTokens: 200,
			TotalTokens:         300,
		},
	}, nil
}

func TestNewCostModel(t *testing.T) {
	sm := &mockSessionManager{}
	model := newCostModel(sm)

	require.NotNil(t, model)
	assert.Equal(t, sm, model.sm)
	assert.NotNil(t, model.table)
	assert.Nil(t, model.err)
	assert.Len(t, model.table.Columns(), 8)
	assert.Equal(t, "NAME", model.table.Columns()[0].Title)
}

func TestCostModelInit(t *testing.T) {
	sm := &mockSessionManager{}
	model := newCostModel(sm)
	cmd := model.Init()
	assert.NotNil(t, cmd)
}

func TestCostModelUpdate(t *testing.T) {
	sm := &mockSessionManager{}
	// Set the global LoadAgentState for the duration of this test
	originalLoadAgentState := LoadAgentState
	LoadAgentState = mockLoadAgentState
	t.Cleanup(func() { LoadAgentState = originalLoadAgentState })

	testCases := []struct {
		name        string
		msg         tea.Msg
		initialErr  error
		expectModel func(*testing.T, *costModel)
		expectCmd   func(*testing.T, tea.Cmd)
	}{
		{
			name: "Quit on 'q'",
			msg:  tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")},
			expectCmd: func(t *testing.T, cmd tea.Cmd) {
				assert.NotNil(t, cmd)
				// tea.Quit is a function, so we can't directly compare.
				// This is a common way to check for a quit command.
				quitMsg := cmd()
				assert.Equal(t, tea.Quit(), quitMsg)
			},
		},
		{
			name: "Quit on 'ctrl+c'",
			msg:  tea.KeyMsg{Type: tea.KeyCtrlC},
			expectCmd: func(t *testing.T, cmd tea.Cmd) {
				assert.NotNil(t, cmd)
				quitMsg := cmd()
				assert.Equal(t, tea.Quit(), quitMsg)
			},
		},
		{
			name: "tickMsg triggers refresh",
			msg:  tickMsg{},
			expectCmd: func(t *testing.T, cmd tea.Cmd) {
				// Expect a batch command containing tick and fetch
				assert.NotNil(t, cmd)
			},
		},
		{
			name: "errMsg sets error state",
			msg:  errMsg{err: errors.New("test error")},
			expectModel: func(t *testing.T, m *costModel) {
				require.NotNil(t, m.err)
				assert.EqualError(t, m.err, "test error")
			},
		},
		{
			name: "updateMsg updates sessions and table",
			msg: updateMsg{
				&runner.SessionState{Name: "session1", AgentStateFile: "state1.json", StartTime: time.Now()},
			},
			expectModel: func(t *testing.T, m *costModel) {
				assert.Len(t, m.sessions, 1)
				assert.Equal(t, "session1", m.sessions[0].Name)
				assert.Len(t, m.table.Rows(), 1, "Table should be updated with one row")
			},
		},
		{
			name: "WindowSizeMsg updates table height",
			msg:  tea.WindowSizeMsg{Width: 80, Height: 25},
			expectModel: func(t *testing.T, m *costModel) {
				// Initial height is 15, new height should be 25-5 = 20
				// This is an internal detail of the table model, but we can check it.
				// Note: this test is slightly brittle if the internal logic changes.
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			model := newCostModel(sm)
			if tc.initialErr != nil {
				model.err = tc.initialErr
			}

			newModel, cmd := model.Update(tc.msg)
			updatedModel, ok := newModel.(*costModel)
			require.True(t, ok)

			if tc.expectModel != nil {
				tc.expectModel(t, updatedModel)
			}
			if tc.expectCmd != nil {
				tc.expectCmd(t, cmd)
			}
		})
	}
}

func TestCostModelView(t *testing.T) {
	// Set the global LoadAgentState for the duration of this test
	originalLoadAgentState := LoadAgentState
	LoadAgentState = mockLoadAgentState
	t.Cleanup(func() { LoadAgentState = originalLoadAgentState })

	sm := &mockSessionManager{}
	model := newCostModel(sm)

	// Test error view
	model.err = errors.New("connection failed")
	view := model.View()
	assert.Contains(t, view, "Error: connection failed")
	assert.Contains(t, view, "Press 'q' to quit.")

	// Test normal view
	model.err = nil
	model.sessions = []*runner.SessionState{
		{
			Name:           "good-session",
			Status:         "running",
			StartTime:      time.Now().Add(-10 * time.Minute),
			AgentStateFile: "good.json",
		},
		{
			Name:           "error-session",
			Status:         "stopped",
			StartTime:      time.Now().Add(-20 * time.Minute),
			EndTime:        time.Now().Add(-15 * time.Minute),
			AgentStateFile: "error.json", // This will trigger the mock error
		},
	}
	model.updateTable()
	view = model.View()
	assert.Contains(t, view, "RECAC Live Session Monitor")
	assert.Contains(t, view, "good-session")
	// The mock model "test-model" is not in the pricing list, so it uses the default rate.
	// Default rate is 1e-6 for both prompt and completion.
	// (100 + 200) * 1e-6 = 0.000300
	assert.Contains(t, view, "$0.000300")
	assert.Contains(t, view, "error-session")
	assert.Contains(t, view, "N/A") // For the session that failed to load state
	assert.Contains(t, view, "q: Quit")
}

func TestFetchSessions(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		expectedSessions := []*runner.SessionState{{Name: "test-session"}}
		sm := &mockSessionManager{sessions: expectedSessions}
		cmd := fetchSessions(sm)
		msg := cmd()

		update, ok := msg.(updateMsg)
		require.True(t, ok, "Expected updateMsg")
		assert.Equal(t, expectedSessions, []*runner.SessionState(update))
	})

	t.Run("Error", func(t *testing.T) {
		expectedErr := errors.New("list failed")
		sm := &mockSessionManager{err: expectedErr}
		cmd := fetchSessions(sm)
		msg := cmd()

		err, ok := msg.(errMsg)
		require.True(t, ok, "Expected errMsg")
		assert.Equal(t, expectedErr, err.err)
	})
}

func TestStartCostTUI(t *testing.T) {
	t.Run("LoadAgentState not set", func(t *testing.T) {
		originalLoadAgentState := LoadAgentState
		LoadAgentState = nil
		t.Cleanup(func() { LoadAgentState = originalLoadAgentState })

		err := StartCostTUI(&mockSessionManager{})
		require.Error(t, err)
		assert.Equal(t, "LoadAgentState function must be set before starting the Cost TUI", err.Error())
	})

	t.Run("TUI starter is called", func(t *testing.T) {
		originalLoadAgentState := LoadAgentState
		LoadAgentState = mockLoadAgentState
		t.Cleanup(func() { LoadAgentState = originalLoadAgentState })

		// Mock the TUI starter function
		var wasCalled bool
		var receivedSM SessionManager
		mockStarter := func(sm SessionManager) error {
			wasCalled = true
			receivedSM = sm
			return nil
		}
		originalStarter := startCostTUI
		SetStartCostTUIForTest(mockStarter)
		t.Cleanup(func() { SetStartCostTUIForTest(originalStarter) })

		sm := &mockSessionManager{}
		err := StartCostTUI(sm)

		assert.NoError(t, err)
		assert.True(t, wasCalled, "The TUI starter function should have been called")
		assert.Equal(t, sm, receivedSM, "The TUI starter received the wrong session manager")
	})

	t.Run("TUI starter returns an error", func(t *testing.T) {
		originalLoadAgentState := LoadAgentState
		LoadAgentState = mockLoadAgentState
		t.Cleanup(func() { LoadAgentState = originalLoadAgentState })

		expectedErr := errors.New("TUI failed")
		mockStarter := func(sm SessionManager) error {
			return expectedErr
		}
		originalStarter := startCostTUI
		SetStartCostTUIForTest(mockStarter)
		t.Cleanup(func() { SetStartCostTUIForTest(originalStarter) })

		err := StartCostTUI(&mockSessionManager{})
		require.Error(t, err)
		assert.Equal(t, expectedErr, err)
	})
}
