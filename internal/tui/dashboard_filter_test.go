package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"recac/internal/orchestrator"
)

func TestDashboardFiltering(t *testing.T) {
	m := NewDashboardModel("http://localhost:2112")

	jobs := []orchestrator.JobInfo{
		{
			ID:        "JOB-1",
			Summary:   "Fix the login bug",
			Status:    "Running",
			StartTime: time.Now(),
		},
		{
			ID:        "JOB-2",
			Summary:   "Add new feature",
			Status:    "Completed",
			StartTime: time.Now().Add(-1 * time.Hour),
		},
	}

	msg := statusMsg{
		Status: orchestrator.Status{},
		Jobs:   jobs,
	}

	model, _ := m.Update(msg)
	dm := model.(DashboardModel)

	// Ensure both rows are visible before filter
	view := dm.View()
	assert.Contains(t, view, "JOB-1")
	assert.Contains(t, view, "Fix the login bug")
	assert.Contains(t, view, "JOB-2")
	assert.Contains(t, view, "Add new feature")

	// Activate filter
	model, _ = dm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	dm = model.(DashboardModel)

	assert.True(t, dm.isFiltering)

	// Type filter text
	model, _ = dm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b', 'u', 'g'}})
	dm = model.(DashboardModel)

	// Check view
	view = dm.View()
	assert.Contains(t, view, "bug") // from input or table
	assert.Contains(t, view, "JOB-1")
	assert.NotContains(t, view, "JOB-2")

	// Cancel filter
	model, _ = dm.Update(tea.KeyMsg{Type: tea.KeyEsc})
	dm = model.(DashboardModel)

	assert.False(t, dm.isFiltering)

	// Check view
	view = dm.View()
	assert.Contains(t, view, "JOB-1")
	assert.Contains(t, view, "JOB-2")
}
