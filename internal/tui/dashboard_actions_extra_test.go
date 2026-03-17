package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"recac/internal/orchestrator"
	"testing"
)

func TestDashboardUpdate_MultipleSelection(t *testing.T) {
	m := NewDashboardModel("http://localhost:2112")

	// Setup some jobs
	m.jobs = []orchestrator.JobInfo{
		{ID: "JOB-1", Summary: "Summary 1"},
		{ID: "JOB-2", Summary: "Summary 2"},
	}
	m.updateTableContent()

	// Select first row via space
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	dm := newM.(DashboardModel)
	assert.True(t, dm.selectedJobs["JOB-1"])

	// Trigger bulk cancel action
	newM, _ = dm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	dm = newM.(DashboardModel)
	assert.Equal(t, viewConfirmation, dm.viewState)
	assert.Equal(t, "cancel multiple", dm.pendingAction)
	assert.Equal(t, "MULTIPLE_c", dm.pendingJobId)

	// Confirm bulk action
	newM, cmd := dm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	dm = newM.(DashboardModel)

	assert.NotNil(t, cmd)
	assert.Equal(t, viewMain, dm.viewState)
	assert.Empty(t, dm.selectedJobs)
}

func TestDashboardUpdate_SelectAll(t *testing.T) {
	m := NewDashboardModel("http://localhost:2112")

	m.jobs = []orchestrator.JobInfo{
		{ID: "JOB-1", Summary: "Summary 1"},
		{ID: "JOB-2", Summary: "Summary 2"},
	}
	m.updateTableContent()

	// Select all via 'v'
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("v")})
	dm := newM.(DashboardModel)
	assert.True(t, dm.selectedJobs["JOB-1"])
	assert.True(t, dm.selectedJobs["JOB-2"])

	// Deselect all via 'v'
	newM, _ = dm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("v")})
	dm = newM.(DashboardModel)
	assert.False(t, dm.selectedJobs["JOB-1"])
	assert.False(t, dm.selectedJobs["JOB-2"])
}
