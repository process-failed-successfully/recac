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
	m := NewStatusDashboardModel("test-session")
	cmd := m.Init()
	assert.NotNil(t, cmd)
}

func TestStatusDashboardModel_Update_Refresh(t *testing.T) {
	m := NewStatusDashboardModel("test-session")

	// Prepare refresh msg
	session := &runner.SessionState{Name: "test-session", Status: "running"}
	state := &agent.State{
		Model: "gpt-4",
		TokenUsage: agent.TokenUsage{TotalTokens: 100},
	}
	msg := statusRefreshedMsg{
		session: session,
		agentState: state,
		gitDiffStat: "mock diff",
	}

	newM, _ := m.Update(msg)
	finalM := newM.(statusDashboardModel)

	assert.Equal(t, session, finalM.session)
	assert.Equal(t, state, finalM.agentState)
	assert.Equal(t, "mock diff", finalM.gitDiffStat)
}

func TestStatusDashboardModel_Update_Error(t *testing.T) {
	m := NewStatusDashboardModel("test-session")
	err := errors.New("fail")
	newM, _ := m.Update(err)
	finalM := newM.(statusDashboardModel)
	assert.Equal(t, err, finalM.err)
}

func TestStatusDashboardModel_Update_WindowSize(t *testing.T) {
	m := NewStatusDashboardModel("test-session")
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	finalM := newM.(statusDashboardModel)
	assert.Equal(t, 100, finalM.width)
	assert.Equal(t, 50, finalM.height)
}

func TestStatusDashboardModel_View(t *testing.T) {
	m := NewStatusDashboardModel("test-session")

	// Error view
	m.err = errors.New("fail")
	assert.Contains(t, m.View(), "Error: fail")
	m.err = nil

	// Loading view
	assert.Contains(t, m.View(), "Loading...")

	// Full view
	m.session = &runner.SessionState{
		Name: "test-session",
		Status: "running",
		StartTime: time.Now().Add(-time.Minute),
		Goal: "test goal",
	}
	m.agentState = &agent.State{
		Model: "gpt-4",
		TokenUsage: agent.TokenUsage{TotalTokens: 100},
		History: []agent.Message{
			{Role: "user", Content: "hello", Timestamp: time.Now()},
		},
	}
	m.gitDiffStat = "1 file changed"
	m.lastUpdate = time.Now()

	view := m.View()
	assert.Contains(t, view, "test-session")
	assert.Contains(t, view, "running")
	assert.Contains(t, view, "test goal")
	assert.Contains(t, view, "gpt-4")
	assert.Contains(t, view, "$") // Cost
	assert.Contains(t, view, "1 file changed")
	assert.Contains(t, view, "hello")
}

func TestRefreshStatusCmd(t *testing.T) {
	// Backup and restore global
	oldFunc := GetSessionStatus
	defer func() { GetSessionStatus = oldFunc }()

	GetSessionStatus = func(name string) (*runner.SessionState, *agent.State, string, error) {
		if name == "fail" {
			return nil, nil, "", errors.New("fail")
		}
		return &runner.SessionState{Name: name}, nil, "", nil
	}

	// Success
	cmd := refreshStatusCmd("success")
	msg := cmd()
	assert.IsType(t, statusRefreshedMsg{}, msg)
	assert.Equal(t, "success", msg.(statusRefreshedMsg).session.Name)

	// Fail
	cmdFail := refreshStatusCmd("fail")
	msgFail := cmdFail()
	assert.Error(t, msgFail.(error))

	// Nil func
	GetSessionStatus = nil
	cmdNil := refreshStatusCmd("any")
	msgNil := cmdNil()
	assert.Error(t, msgNil.(error))
}
