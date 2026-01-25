package ui

import (
	"errors"
	"testing"
	"time"

	"recac/internal/agent"
	"recac/internal/runner"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestStatusDashboardModel_Init(t *testing.T) {
	m := NewStatusDashboardModel("session1")
	cmd := m.Init()
	assert.NotNil(t, cmd)
}

func TestStatusDashboardModel_Update_WindowSize(t *testing.T) {
	m := NewStatusDashboardModel("session1")

	msg := tea.WindowSizeMsg{Width: 100, Height: 50}
	newM, cmd := m.Update(msg)

	updatedM := newM.(statusDashboardModel)
	assert.Equal(t, 100, updatedM.width)
	assert.Equal(t, 50, updatedM.height)
	assert.Nil(t, cmd)
}

func TestStatusDashboardModel_Update_StatusRefreshed(t *testing.T) {
	m := NewStatusDashboardModel("session1")

	session := &runner.SessionState{Status: "running"}
	agentState := &agent.State{Model: "gpt-4"}
	msg := statusRefreshedMsg{
		session:     session,
		agentState:  agentState,
		gitDiffStat: "diff",
	}

	newM, cmd := m.Update(msg)

	updatedM := newM.(statusDashboardModel)
	assert.Equal(t, session, updatedM.session)
	assert.Equal(t, agentState, updatedM.agentState)
	assert.Equal(t, "diff", updatedM.gitDiffStat)
	assert.False(t, updatedM.lastUpdate.IsZero())
	assert.Nil(t, cmd)
}

func TestStatusDashboardModel_Update_Error(t *testing.T) {
	m := NewStatusDashboardModel("session1")

	err := errors.New("update failed")
	newM, cmd := m.Update(err)

	updatedM := newM.(statusDashboardModel)
	assert.Equal(t, err, updatedM.err)
	assert.Nil(t, cmd)
}

func TestStatusDashboardModel_View_Error(t *testing.T) {
	m := NewStatusDashboardModel("session1")
	m.err = errors.New("some error")

	output := m.View()
	assert.Contains(t, output, "Error: some error")
}

func TestStatusDashboardModel_View_Loading(t *testing.T) {
	m := NewStatusDashboardModel("session1")
	// m.session is nil by default

	output := m.View()
	assert.Contains(t, output, "Loading...")
}

func TestStatusDashboardModel_View_Content(t *testing.T) {
	m := NewStatusDashboardModel("session1")
	m.session = &runner.SessionState{
		Name:      "session1",
		Status:    "running",
		Goal:      "Test Goal",
		StartTime: time.Now().Add(-1 * time.Hour),
	}
	m.agentState = &agent.State{
		Model: "gpt-4",
		TokenUsage: agent.TokenUsage{
			TotalTokens: 100,
		},
		History: []agent.Message{
			{Role: "user", Content: "hello", Timestamp: time.Now()},
		},
	}
	m.width = 80

	output := m.View()

	assert.Contains(t, output, "RECAC Session Status: session1")
	assert.Contains(t, output, "Status")
	assert.Contains(t, output, "running")
	assert.Contains(t, output, "Test Goal")
	assert.Contains(t, output, "gpt-4")
	assert.Contains(t, output, "Tokens")
	assert.Contains(t, output, "hello")
}

func TestRefreshStatusCmd(t *testing.T) {
	// Mock the global GetSessionStatus
	originalFunc := GetSessionStatus
	defer func() { GetSessionStatus = originalFunc }()

	called := false
	GetSessionStatus = func(sessionName string) (*runner.SessionState, *agent.State, string, error) {
		called = true
		assert.Equal(t, "test-session", sessionName)
		return &runner.SessionState{}, &agent.State{}, "diff", nil
	}

	cmd := refreshStatusCmd("test-session")
	msg := cmd()

	assert.True(t, called)
	assert.IsType(t, statusRefreshedMsg{}, msg)
}

func TestRefreshStatusCmd_Error(t *testing.T) {
	// Mock the global GetSessionStatus
	originalFunc := GetSessionStatus
	defer func() { GetSessionStatus = originalFunc }()

	GetSessionStatus = func(sessionName string) (*runner.SessionState, *agent.State, string, error) {
		return nil, nil, "", errors.New("fail")
	}

	cmd := refreshStatusCmd("test-session")
	msg := cmd()

	assert.Equal(t, errors.New("fail"), msg)
}
