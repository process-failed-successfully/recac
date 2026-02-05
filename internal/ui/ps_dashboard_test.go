package ui

import (
	"errors"
	"recac/internal/agent"
	"recac/internal/model"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestPsDashboardModel_Init(t *testing.T) {
	// Set mock GetSessions
	oldGetSessions := GetSessions
	defer func() { GetSessions = oldGetSessions }()
	GetSessions = func() ([]model.UnifiedSession, error) { return nil, nil }

	m := NewPsDashboardModel(false, "time")
	cmd := m.Init()
	assert.NotNil(t, cmd)
}

func TestPsDashboardModel_Update_Refresh(t *testing.T) {
	// Setup sessions
	sessions := []model.UnifiedSession{
		{Name: "session1", Status: "running", StartTime: time.Now().Add(-1 * time.Hour), Cost: 10.0, HasCost: true},
		{Name: "session2", Status: "running", StartTime: time.Now(), Cost: 5.0, HasCost: true},
	}

	m := NewPsDashboardModel(true, "cost")

	updatedM, cmd := m.Update(psSessionsRefreshedMsg(sessions))
	m = updatedM.(psDashboardModel)

	assert.Nil(t, cmd)
	assert.Equal(t, 2, len(m.sessions))

	// Verify sorting (by cost descending)
	assert.Equal(t, "session1", m.sessions[0].Name)
	assert.Equal(t, "session2", m.sessions[1].Name)
}

func TestPsDashboardModel_Update_SortByName(t *testing.T) {
	sessions := []model.UnifiedSession{
		{Name: "B"},
		{Name: "A"},
	}

	m := NewPsDashboardModel(false, "name")

	updatedM, _ := m.Update(psSessionsRefreshedMsg(sessions))
	m = updatedM.(psDashboardModel)

	assert.Equal(t, "A", m.sessions[0].Name)
	assert.Equal(t, "B", m.sessions[1].Name)
}

func TestPsDashboardModel_Update_Tick(t *testing.T) {
	oldGetSessions := GetSessions
	defer func() { GetSessions = oldGetSessions }()
	called := false
	GetSessions = func() ([]model.UnifiedSession, error) {
		called = true
		return nil, nil
	}

	m := NewPsDashboardModel(false, "time")

	// Execute command returned by Tick handling?
	// The Update returns a cmd which is `refreshPsSessionsCmd`.
	updatedM, cmd := m.Update(psTickMsg(time.Now()))
	m = updatedM.(psDashboardModel)
	assert.NotNil(t, cmd)

	cmd() // Execute refresh
	assert.True(t, called)
}

func TestPsDashboardModel_Update_Error(t *testing.T) {
	m := NewPsDashboardModel(false, "time")
	err := errors.New("test error")

	updatedM, _ := m.Update(err)
	m = updatedM.(psDashboardModel)

	assert.Equal(t, err, m.err)
	assert.Contains(t, m.View(), "test error")
}

func TestPsDashboardModel_View(t *testing.T) {
	m := NewPsDashboardModel(true, "time")
	m.sessions = []model.UnifiedSession{
		{
			Name: "s1", Status: "running",
			CPU: "10%", Memory: "100MB", Location: "local",
			Tokens: agent.TokenUsage{TotalTokens: 100},
			Cost: 0.1, HasCost: true,
		},
	}
	m.updateTableRows()

	// Set window size (Needs to be wide enough to show all columns)
	updatedM, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 24})
	m = updatedM.(psDashboardModel)

	view := m.View()
	assert.Contains(t, view, "RECAC PS Dashboard")
	assert.Contains(t, view, "s1")
	assert.Contains(t, view, "10%")
	assert.Contains(t, view, "$0.100000")
}
