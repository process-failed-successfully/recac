package ui

import (
	"errors"
	"recac/internal/agent"
	"recac/internal/runner"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockSessionManager implements SessionManager for testing
type MockSessionManager struct {
	mock.Mock
}

func (m *MockSessionManager) ListSessions() ([]*runner.SessionState, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*runner.SessionState), args.Error(1)
}

func TestStartCostTUI(t *testing.T) {
	// Restore LoadAgentState after test
	defer func() { LoadAgentState = nil }()

	sm := new(MockSessionManager)

	// Test case: LoadAgentState not set
	LoadAgentState = nil
	err := StartCostTUI(sm)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "LoadAgentState function must be set")

	// Test case: Success (mocked)
	LoadAgentState = func(filePath string) (*agent.State, error) {
		return nil, nil
	}

	// We can't easily test the full tea.Program.Start() in a unit test without it seizing the terminal,
	// but we can at least verify the guard clause above.
	// For full TUI testing, we typically test the model's Update/View methods directly as done below.
}

func TestCostModel_Init(t *testing.T) {
	sm := new(MockSessionManager)
	model := newCostModel(sm)

	cmd := model.Init()
	assert.NotNil(t, cmd)

	// Init should batch tickCmd and fetchSessions
	// Executing the batch command involves internal BubbleTea logic,
	// but we can verify it's a BatchMsg by running it? No, tea.Batch returns a tea.Cmd.
	// The tea.Cmd is a function.
}

func TestCostModel_Update(t *testing.T) {
	sm := new(MockSessionManager)
	model := newCostModel(sm)

	// Test Quit
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}
	_, cmd := model.Update(msg)
	assert.Equal(t, tea.Quit(), cmd())

	// Test Ctrl+C
	msg = tea.KeyMsg{Type: tea.KeyCtrlC}
	_, cmd = model.Update(msg)
	assert.Equal(t, tea.Quit(), cmd())

	// Test WindowSizeMsg
	sizeMsg := tea.WindowSizeMsg{Width: 100, Height: 50}
	updatedModel, _ := model.Update(sizeMsg)
	m := updatedModel.(*costModel)
	assert.Equal(t, 43, m.table.Height())

	// Test errMsg
	testErr := errors.New("test error")
	updatedModel, _ = model.Update(errMsg{err: testErr})
	m = updatedModel.(*costModel)
	assert.Equal(t, testErr, m.err)

	// Test tickMsg
	tick := tickMsg(time.Now())
	_, cmd = model.Update(tick)
	assert.NotNil(t, cmd)
}

func TestCostModel_Update_UpdateMsg(t *testing.T) {
	sm := new(MockSessionManager)
	model := newCostModel(sm)

	// Setup mock data
	startTime := time.Now().Add(-1 * time.Hour)
	sessions := []*runner.SessionState{
		{
			Name:           "session1",
			Status:         "running",
			StartTime:      startTime,
			AgentStateFile: "state.json",
		},
		{
			Name:           "session2",
			Status:         "completed",
			StartTime:      startTime,
			EndTime:        time.Now(),
			AgentStateFile: "state2.json",
		},
	}

	// Mock LoadAgentState
	LoadAgentState = func(filePath string) (*agent.State, error) {
		if filePath == "state.json" {
			return &agent.State{
				Model: "gpt-4",
				TokenUsage: agent.TokenUsage{
					TotalPromptTokens:   100,
					TotalResponseTokens: 200,
					TotalTokens:         300,
				},
			}, nil
		}
		return nil, errors.New("not found")
	}
	defer func() { LoadAgentState = nil }()

	// Update model with sessions
	msg := updateMsg(sessions)
	updatedModel, _ := model.Update(msg)
	m := updatedModel.(*costModel)

	assert.Len(t, m.sessions, 2)
	assert.NoError(t, m.err)

	// Verify table rows
	rows := m.table.Rows()
	assert.Len(t, rows, 2)

	// Verify row content for session 1 (with valid agent state)
	assert.Equal(t, "session1", rows[0][0])
	assert.Equal(t, "running", rows[0][1])
	assert.Equal(t, "100", rows[0][4]) // Prompt tokens
	assert.Equal(t, "200", rows[0][5]) // Response tokens
	assert.Equal(t, "300", rows[0][6]) // Total tokens

	// Verify row content for session 2 (error loading agent state)
	assert.Equal(t, "session2", rows[1][0])
	assert.Equal(t, "completed", rows[1][1])
	assert.Equal(t, "N/A", rows[1][4]) // Prompt tokens
}

func TestCostModel_View(t *testing.T) {
	sm := new(MockSessionManager)
	model := newCostModel(sm)

	// Test Normal View
	view := model.View()
	assert.Contains(t, view, "RECAC Live Session Monitor")
	assert.Contains(t, view, "NAME")
	assert.Contains(t, view, "COST")

	// Test Error View
	model.err = errors.New("some error")
	view = model.View()
	assert.Contains(t, view, "Error: some error")
}

func TestFetchSessions(t *testing.T) {
	sm := new(MockSessionManager)

	// Test success
	expectedSessions := []*runner.SessionState{{Name: "test"}}
	sm.On("ListSessions").Return(expectedSessions, nil).Once()

	cmd := fetchSessions(sm)
	msg := cmd()
	assert.IsType(t, updateMsg{}, msg)
	assert.Equal(t, updateMsg(expectedSessions), msg)

	// Test failure
	expectedErr := errors.New("fail")
	sm.On("ListSessions").Return(nil, expectedErr).Once()

	cmd = fetchSessions(sm)
	msg = cmd()
	assert.IsType(t, errMsg{}, msg)
	assert.Equal(t, errMsg{expectedErr}, msg)
}

// TestSetStartCostTUIForTest ensures we can replace the starter function
func TestSetStartCostTUIForTest(t *testing.T) {
	called := false
	mockStart := func(sm SessionManager) error {
		called = true
		return nil
	}

	originalStart := startCostTUI
	defer func() { startCostTUI = originalStart }()

	SetStartCostTUIForTest(mockStart)

	err := StartCostTUI(nil)
	assert.NoError(t, err)
	assert.True(t, called)
}
