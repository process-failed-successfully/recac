package ui

import (
	"errors"
	"recac/internal/agent"
	"recac/internal/runner"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestStatusDashboardModel_Init(t *testing.T) {
	oldGetSessionStatus := GetSessionStatus
	defer func() { GetSessionStatus = oldGetSessionStatus }()
	GetSessionStatus = func(sessionName string) (*runner.SessionState, *agent.State, string, error) {
		return nil, nil, "", nil
	}

	m := NewStatusDashboardModel("test-session")
	cmd := m.Init()
	assert.NotNil(t, cmd)
}

func TestStatusDashboardModel_Update_Refresh(t *testing.T) {
	session := &runner.SessionState{
		Status: "running",
		Goal: "Test Goal",
		StartTime: time.Now(),
	}
	agentState := &agent.State{
		Model: "gemini-pro",
		TokenUsage: agent.TokenUsage{TotalTokens: 100},
	}
	gitDiff := "1 file changed"

	m := NewStatusDashboardModel("test-session")

	msg := statusRefreshedMsg{
		session: session,
		agentState: agentState,
		gitDiffStat: gitDiff,
	}

	updatedM, cmd := m.Update(msg)
	m = updatedM.(statusDashboardModel)

	assert.Nil(t, cmd)
	assert.Equal(t, session, m.session)
	assert.Equal(t, agentState, m.agentState)
	assert.Equal(t, gitDiff, m.gitDiffStat)
}

func TestStatusDashboardModel_Update_Tick(t *testing.T) {
	oldGetSessionStatus := GetSessionStatus
	defer func() { GetSessionStatus = oldGetSessionStatus }()
	called := false
	GetSessionStatus = func(sessionName string) (*runner.SessionState, *agent.State, string, error) {
		called = true
		return nil, nil, "", nil
	}

	m := NewStatusDashboardModel("test-session")

	updatedM, cmd := m.Update(statusTickMsg(time.Now()))
	m = updatedM.(statusDashboardModel)
	assert.NotNil(t, cmd)

	// Execute refresh command directly to verify callback
	refreshCmd := refreshStatusCmd("test-session")
	refreshCmd()
	assert.True(t, called)
}

func TestStatusDashboardModel_Update_Error(t *testing.T) {
	m := NewStatusDashboardModel("test-session")
	err := errors.New("test error")

	updatedM, _ := m.Update(err)
	m = updatedM.(statusDashboardModel)

	assert.Equal(t, err, m.err)
	assert.Contains(t, m.View(), "test error")
}

func TestStatusDashboardModel_View(t *testing.T) {
	m := NewStatusDashboardModel("test-session")

	// Case 1: Loading
	assert.Contains(t, m.View(), "Loading...")

	// Case 2: Data available
	m.session = &runner.SessionState{
		Status: "running",
		Goal: "Test Goal",
		StartTime: time.Now(),
	}
	m.agentState = &agent.State{
		Model: "gemini-pro",
		TokenUsage: agent.TokenUsage{TotalTokens: 100, TotalPromptTokens: 50, TotalResponseTokens: 50},
		History: []agent.Message{
			{Role: "user", Content: "Hello", Timestamp: time.Now()},
		},
	}
	m.gitDiffStat = "1 file changed"

	// Set window size
	updatedM, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	m = updatedM.(statusDashboardModel)

	view := m.View()
	assert.Contains(t, view, "RECAC Session Status: test-session")
	assert.Contains(t, view, "Status:")
	assert.Contains(t, view, "running")
	assert.Contains(t, view, "Model:")
	assert.Contains(t, view, "gemini-pro")
	assert.Contains(t, view, "Cost:")
	assert.Contains(t, view, "Git Changes")
	assert.Contains(t, view, "1 file changed")
	assert.Contains(t, view, "Last Activity")
	assert.Contains(t, view, "Hello")
}
