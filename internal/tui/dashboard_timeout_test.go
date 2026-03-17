package tui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"recac/internal/orchestrator"
)

func TestDashboard_TimeoutUpdateCmd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/jobs/TEST-TIMEOUT/timeout", r.URL.Path)

		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"timeout": "30m"}`)
	}))
	defer server.Close()

	cmd := updateTimeoutCmd(server.URL, "TEST-TIMEOUT", "30m")
	msg := cmd()

	actionMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.NoError(t, actionMsg.Err)
	assert.Contains(t, actionMsg.Message, "Updated timeout for job TEST-TIMEOUT to 30m")
}

func TestDashboard_TimeoutUpdateKeys(t *testing.T) {
	m := NewDashboardModel("http://dummy")

	// Setup a job and table
	m.jobs = []orchestrator.JobInfo{
		{
			ID: "TEST-TIMEOUT",
			WorkItem: orchestrator.WorkItem{
				Priority: 5,
			},
			StartTime: time.Now(),
		},
	}
	m.updateTableContent()
	m.table.SetCursor(0)

	// Trigger timeout update mode via 'T'
	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'T'}})
	m = m2.(DashboardModel)

	assert.NotNil(t, cmd)
	assert.Equal(t, viewTimeoutInput, m.viewState)
	assert.Equal(t, "TEST-TIMEOUT", m.pendingJobId)
	assert.True(t, m.timeoutInput.Focused())

	// Type "2h"
	m2, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	m = m2.(DashboardModel)
	m2, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	m = m2.(DashboardModel)

	assert.Equal(t, "2h", m.timeoutInput.Value())

	// Submit via Enter
	m2, submitCmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = m2.(DashboardModel)

	assert.NotNil(t, submitCmd)
	assert.Equal(t, viewMain, m.viewState)
	assert.Equal(t, "", m.pendingJobId)
	assert.False(t, m.timeoutInput.Focused())
}

func TestDashboard_TimeoutUpdateCancel(t *testing.T) {
	m := NewDashboardModel("http://dummy")

	m.jobs = []orchestrator.JobInfo{
		{ID: "TEST-TIMEOUT", StartTime: time.Now()},
	}
	m.updateTableContent()
	m.table.SetCursor(0)

	// Enter timeout update mode
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'T'}})
	m = m2.(DashboardModel)
	assert.Equal(t, viewTimeoutInput, m.viewState)

	// Cancel via Esc
	m2, cancelCmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = m2.(DashboardModel)

	assert.Nil(t, cancelCmd) // cancel should return no command
	assert.Equal(t, viewMain, m.viewState)
	assert.Equal(t, "", m.pendingJobId)
}

func TestDashboard_TimeoutUpdateMultipleKeys(t *testing.T) {
	m := NewDashboardModel("http://dummy")

	// Setup multiple jobs
	m.jobs = []orchestrator.JobInfo{
		{ID: "TEST-TIMEOUT-1", StartTime: time.Now()},
		{ID: "TEST-TIMEOUT-2", StartTime: time.Now()},
	}
	m.updateTableContent()

	// Select both jobs
	m.selectedJobs = map[string]bool{
		"TEST-TIMEOUT-1": true,
		"TEST-TIMEOUT-2": true,
	}

	// Trigger timeout update mode via 'T'
	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'T'}})
	m = m2.(DashboardModel)

	assert.NotNil(t, cmd)
	assert.Equal(t, viewTimeoutInput, m.viewState)
	assert.Equal(t, "MULTIPLE_timeout", m.pendingJobId)
	assert.True(t, m.timeoutInput.Focused())

	// Type "1h"
	m2, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	m = m2.(DashboardModel)
	m2, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	m = m2.(DashboardModel)

	assert.Equal(t, "1h", m.timeoutInput.Value())

	// Submit via Enter
	m2, submitCmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = m2.(DashboardModel)

	assert.NotNil(t, submitCmd)
	assert.Equal(t, viewMain, m.viewState)
	assert.Equal(t, "", m.pendingJobId)
	assert.False(t, m.timeoutInput.Focused())

	// Verify the batch command handles the multiple jobs.
	// `tea.Batch` returns a `tea.BatchMsg` holding a slice of commands.
	batchMsg := submitCmd()
	batchMsgs, ok := batchMsg.(tea.BatchMsg)
	assert.True(t, ok, "Expected a batch of commands for multiple selected jobs")
	assert.Equal(t, 2, len(batchMsgs), "Expected 2 commands in the batch")

	// Verify selection was cleared
	assert.Empty(t, m.selectedJobs)
}
