package ui

import (
	"recac/internal/agent"
	"recac/internal/model"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestPsDashboardModel_DetailsView_EnterAndEsc(t *testing.T) {
	now := time.Now()
	m := NewPsDashboardModel(false, "time")
	m.sessions = []model.UnifiedSession{
		{Name: "session-1", Status: "Running", Goal: "Goal 1", LastActivity: now, Location: "local"},
		{Name: "session-2", Status: "Stopped", Goal: "Goal 2", LastActivity: now, Location: "k8s"},
	}
	m.updateTableRows()

	// Initial state: showing table, not details
	assert.False(t, m.showDetails)

	// Simulate pressing 'Enter' on first row (index 0)
	// m.table.Cursor() defaults to 0
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedM, _ := m.Update(msg)
	modelVal := updatedM.(psDashboardModel)

	assert.True(t, modelVal.showDetails)
	assert.Equal(t, "session-1", modelVal.selectedSession.Name)

	// Verify View shows details
	view := modelVal.View()
	assert.Contains(t, view, "Session Details: session-1")
	assert.Contains(t, view, "Goal 1")
	assert.Contains(t, view, "press 'esc' to back")

	// Simulate pressing 'Esc' to go back
	msg = tea.KeyMsg{Type: tea.KeyEsc}
	updatedM, _ = modelVal.Update(msg)
	modelVal = updatedM.(psDashboardModel)

	assert.False(t, modelVal.showDetails)

	// Verify View shows table again
	view = modelVal.View()
	assert.Contains(t, view, "RECAC PS Dashboard")
	assert.Contains(t, view, "session-1") // Table content
	assert.NotContains(t, view, "Session Details:")
}

func TestPsDashboardModel_DetailsView_Content(t *testing.T) {
	now := time.Now()
	m := NewPsDashboardModel(true, "time") // Show costs true
	session := model.UnifiedSession{
		Name:         "rich-session",
		Status:       "Running",
		Goal:         "A very detailed goal that spans multiple lines or is just long.",
		LastActivity: now,
		StartTime:    now.Add(-time.Hour),
		Location:     "local",
		CPU:          "10.5%",
		Memory:       "256MB",
		HasCost:      true,
		Cost:         1.234567,
		Tokens:       agent.TokenUsage{TotalTokens: 1000},
	}

	m.sessions = []model.UnifiedSession{session}
	m.selectedSession = session
	m.showDetails = true

	view := m.View()

	assert.Contains(t, view, "rich-session")
	assert.Contains(t, view, "Status: Running")
	assert.Contains(t, view, "Location: local")
	assert.Contains(t, view, "CPU: 10.5%")
	assert.Contains(t, view, "Memory: 256MB")
	assert.Contains(t, view, "$1.234567")
	assert.Contains(t, view, "Total Tokens: 1000")
	assert.Contains(t, view, "A very detailed goal")
}
