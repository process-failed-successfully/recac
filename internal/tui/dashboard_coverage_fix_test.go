package tui

import (
	"recac/internal/orchestrator"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDashboardUpdate_Tick(t *testing.T) {
	m := NewDashboardModel("http://localhost:8080")

	// Send tickMsg
	newModel, cmd := m.Update(tickMsg(time.Now()))

	// Should return a batch command
	assert.NotNil(t, cmd)

	// Verify it remains a DashboardModel
	_, ok := newModel.(DashboardModel)
	assert.True(t, ok)
}

func TestDashboardUpdate_StatusMsg(t *testing.T) {
	m := NewDashboardModel("http://localhost:8080")

	// 1. Success
	jobs := []orchestrator.JobInfo{
		{ID: "1", Summary: "Job 1", StartTime: time.Now()},
	}
	status := orchestrator.Status{ActiveSpawns: 1}

	msg := statusMsg{Status: status, Jobs: jobs}
	newModel, _ := m.Update(msg)
	dm := newModel.(DashboardModel)

	assert.Nil(t, dm.err)
	assert.Equal(t, 1, dm.status.ActiveSpawns)
	assert.Len(t, dm.jobs, 1)
	// Check table updated
	assert.NotEmpty(t, dm.table.Rows())

	// 2. Error
	errMsg := statusMsg{Err: assert.AnError}
	newModel, _ = m.Update(errMsg)
	dm = newModel.(DashboardModel)
	assert.Equal(t, assert.AnError, dm.err)
}

func TestDashboardUpdate_DetailsMsg(t *testing.T) {
	m := NewDashboardModel("http://localhost:8080")

	// Success
	job := orchestrator.JobInfo{ID: "1", Summary: "Job 1"}
	msg := detailsMsg{Job: job}

	newModel, _ := m.Update(msg)
	dm := newModel.(DashboardModel)

	assert.Equal(t, viewDetails, dm.viewState)
	assert.Equal(t, job, dm.details)

	// Error
	errMsg := detailsMsg{Err: assert.AnError}
	newModel, _ = m.Update(errMsg)
	dm = newModel.(DashboardModel)
	assert.Equal(t, assert.AnError, dm.err)
}

func TestDashboardView(t *testing.T) {
	m := NewDashboardModel("http://localhost:8080")
	m.status = orchestrator.Status{
		Uptime: "1h",
		LastPoll: time.Now(),
	}

	// Main View
	m.viewState = viewMain
	output := m.View()
	assert.Contains(t, output, "Orchestrator Dashboard")
	assert.Contains(t, output, "Host: http://localhost:8080")

	// Details View
	m.viewState = viewDetails
	m.details = orchestrator.JobInfo{ID: "123", Summary: "Detail Job"}
	m.viewport.SetContent(renderDetails(m.details)) // Set content manually as Update does
	output = m.View()
	assert.Contains(t, output, "ID: 123")

	// Error
	m.err = assert.AnError
	output = m.View()
	assert.Contains(t, output, "Error:")

	// Quitting
	m.quitting = true
	output = m.View()
	assert.Contains(t, output, "Exiting dashboard")
}
