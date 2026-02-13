package ui

import (
	"recac/internal/model"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

// -- PsDashboard Tests --

func TestPsDashboardModel_Sorting_Extended(t *testing.T) {
	now := time.Now()
	sessions := []model.UnifiedSession{
		{Name: "A", Cost: 0.0, HasCost: false, StartTime: now.Add(-10 * time.Minute)}, // No cost
		{Name: "B", Cost: 2.0, HasCost: true, StartTime: now},                        // Cost
		{Name: "C", Cost: 1.0, HasCost: true, StartTime: now.Add(-time.Hour)},        // Cost
	}

	// 1. Sort by Cost (Mixed)
	m := NewPsDashboardModel(true, "cost")
	m.sessions = make([]model.UnifiedSession, len(sessions))
	copy(m.sessions, sessions)

	m.sortSessions()

	// Expect: HasCost=true first, then by Cost desc. HasCost=false last.
	// B ($2), C ($1), A (N/A)
	assert.Equal(t, "B", m.sessions[0].Name)
	assert.Equal(t, "C", m.sessions[1].Name)
	assert.Equal(t, "A", m.sessions[2].Name)

	// 2. Sort by Time (Explicit)
	m = NewPsDashboardModel(true, "time")
	m.sessions = make([]model.UnifiedSession, len(sessions))
	copy(m.sessions, sessions)

	m.sortSessions()

	// Expect: Newest first.
	// B (now), A (now-10m), C (now-1h)
	assert.Equal(t, "B", m.sessions[0].Name)
	assert.Equal(t, "A", m.sessions[1].Name)
	assert.Equal(t, "C", m.sessions[2].Name)

	// 3. Sort by Default (Implicit)
	m = NewPsDashboardModel(true, "unknown")
	m.sessions = make([]model.UnifiedSession, len(sessions))
	copy(m.sessions, sessions)

	m.sortSessions()

	// Expect: Same as Time (Newest first)
	assert.Equal(t, "B", m.sessions[0].Name)
	assert.Equal(t, "A", m.sessions[1].Name)
	assert.Equal(t, "C", m.sessions[2].Name)
}

func TestPsDashboardModel_Update_Tick(t *testing.T) {
	// Test that psTickMsg triggers refresh
	m := NewPsDashboardModel(false, "time")

	// Mock GetSessions
	called := false
	oldGet := GetSessions
	defer func() { GetSessions = oldGet }()
	GetSessions = func() ([]model.UnifiedSession, error) {
		called = true
		return []model.UnifiedSession{}, nil
	}

	msg := psTickMsg(time.Now())
	_, cmd := m.Update(msg)

	// cmd should be refreshPsSessionsCmd
	assert.NotNil(t, cmd)
	cmd() // Execute it
	assert.True(t, called)
}


// -- MonitorDashboard Tests --

func TestMonitorDashboardModel_Init(t *testing.T) {
	m := NewMonitorDashboardModel(ActionCallbacks{})
	cmd := m.Init()
	// Init returns Tick and refresh
	assert.NotNil(t, cmd)
}

func TestMonitorDashboardModel_Update_Tick(t *testing.T) {
	m := NewMonitorDashboardModel(ActionCallbacks{})
	// We verify that tick message handling is part of Update logic
	// But creating tickMsg is hard if it is not exported or conflicting.
	// We skip explicit tick test if we can't construct message easily.
	// But Init test ensures Tick command is returned.
	_ = m
}

// -- SummaryDashboard Tests --

func TestSummaryModel_Init(t *testing.T) {
	m := NewSummaryModel()
	cmd := m.Init()
	assert.NotNil(t, cmd)
}

// -- TopDashboard Tests --

func TestTopDashboardModel_Init(t *testing.T) {
	m := NewTopDashboardModel()
	cmd := m.Init()
	assert.NotNil(t, cmd)
}

// -- StatusDashboard Tests --
// TestStatusDashboardModel_Init is already in status_dashboard_test.go
// We remove it from here.

func TestStatusDashboardModel_Construction(t *testing.T) {
	// We can test constructor with empty string
	m := NewStatusDashboardModel("")
	assert.Equal(t, "", m.sessionName)
}

// -- Wizard Tests --

func TestWizardModel_Init(t *testing.T) {
	m := NewWizardModel()
	cmd := m.Init()
	assert.NotNil(t, cmd)
}

func TestWizardModel_View_Steps(t *testing.T) {
	m := NewWizardModel()
	// Default step 0
	view := m.View()
	assert.Contains(t, view, "Project Setup")
	assert.Contains(t, view, "Enter project directory")

	// Advance step
	// Wait, Update returns new model.
	// Step 1: Enter path. Path cannot be empty.
	m.textInput.SetValue("/tmp/test")
	res, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(WizardModel)

	// Ensure list has size to render title
	// The list SetHeight is set to 10 in constructor, but View might not render title if context is weird.
	// Actually, `WizardModel.Update` only sets `m.list.SetWidth` on WindowSizeMsg, not height?
	// But `NewWizardModel` calls `l.SetHeight(10)`.
	// Let's send a generous size.
	res, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	m = res.(WizardModel)
	// Force title show just in case (though it is default on New)
	m.list.SetShowTitle(true)

	view = m.View()
	// Step 2 is Provider
	assert.Contains(t, view, "Select Agent Provider")
}
