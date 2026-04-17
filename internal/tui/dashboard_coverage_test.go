package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"recac/internal/orchestrator"
	"testing"
	"time"
)

func TestDashboardModel_UpdateSubmitCoverage(t *testing.T) {
	m := NewDashboardModel("http://localhost")
	m.viewState = viewSubmit

	// Test ctrl+s (submit form)
	m.inputs[0].SetValue("summary1")
	m.inputs[1].SetValue("repo1")
	m.inputs[2].SetValue("dep1, dep2")
	m.inputs[3].SetValue("group1")
	m.inputs[4].SetValue("true")
	m.inputs[5].SetValue("tag1, tag2")
	m.inputs[6].SetValue("provider1")
	m.inputs[7].SetValue("model1")
	m.textarea.SetValue("desc1")

	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	model := newM.(DashboardModel)
	assert.NotNil(t, cmd)
	assert.Equal(t, viewMain, model.viewState)

	// Reset to submit
	m.viewState = viewSubmit
	m.inputs[0].SetValue("")

	// Test enter to switch focus
	m.focusedInput = 0
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = newM.(DashboardModel)
	assert.Equal(t, 1, model.focusedInput)

	// Test up/shift+tab to focus previous
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyUp})
	model = newM.(DashboardModel)
	assert.Equal(t, 0, model.focusedInput)
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	model = newM.(DashboardModel)
	assert.Equal(t, len(m.inputs), model.focusedInput) // loops to textarea

	// Test down/tab to focus next
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown}) // from textarea - special case where it ignores down
	model = newM.(DashboardModel)
	assert.Equal(t, len(m.inputs), model.focusedInput) // should stay on textarea

	// Tab behaves differently when navigating logic.
	m.focusedInput = -1
	// simulate shift+tab wrapping backwards? Actually -1 sets to len(m.inputs).
	newM2, _ := m.updateSubmit(tea.KeyMsg{Type: tea.KeyShiftTab, Alt: false})
	assert.Equal(t, len(m.inputs), newM2.focusedInput)

	m.focusedInput = len(m.inputs) + 1
	newM3, _ := m.updateSubmit(tea.KeyMsg{Type: tea.KeyTab, Alt: false})
	assert.Equal(t, 0, newM3.focusedInput)

	m.focusedInput = len(m.inputs)
	newM4, _ := m.updateSubmit(tea.KeyMsg{Type: tea.KeyEnter, Alt: false})
	assert.Equal(t, len(m.inputs), newM4.focusedInput) // ignores enter in textarea for focus change
}

func TestDashboardModel_RenderCompareCoverage(t *testing.T) {
	now := time.Now()
	job1 := orchestrator.JobInfo{
		ID:        "JOB-1",
		Summary:   "Sum 1",
		Status:    "Completed",
		StartTime: now.Add(-2 * time.Hour),
		EndTime:   now,
		WorkItem:  orchestrator.WorkItem{AgentProvider: "prov1", AgentModel: "mod1"},
		Outputs:   map[string]string{"OUT1": "val1", "OUT_SAME": "same"},
		Metrics:   map[string]float64{"MET1": 1.0, "MET_SAME": 3.14},
	}
	job2 := orchestrator.JobInfo{
		ID:        "JOB-2",
		Summary:   "Sum 2",
		Status:    "Failed",
		StartTime: now.Add(-1 * time.Hour),
		EndTime:   time.Time{}, // running
		WorkItem:  orchestrator.WorkItem{AgentProvider: "prov2", AgentModel: "mod2"},
		Outputs:   map[string]string{"OUT2": "val2", "OUT_SAME": "same"},
		Metrics:   map[string]float64{"MET2": 2.0, "MET_SAME": 3.14},
	}

	out := renderCompare(job1, job2)
	assert.Contains(t, out, "JOB-1")
	assert.Contains(t, out, "JOB-2")
	assert.Contains(t, out, "Sum 1")
	assert.Contains(t, out, "Sum 2")
	assert.Contains(t, out, "OUT1")
	assert.Contains(t, out, "OUT2")
	assert.Contains(t, out, "OUT_SAME")
	assert.Contains(t, out, "MET1")
	assert.Contains(t, out, "MET2")
	assert.Contains(t, out, "MET_SAME")

	// no outputs / metrics
	jobEmpty1 := orchestrator.JobInfo{ID: "JOB-3"}
	jobEmpty2 := orchestrator.JobInfo{ID: "JOB-4"}
	outEmpty := renderCompare(jobEmpty1, jobEmpty2)
	assert.Contains(t, outEmpty, "No outputs for either job.")
	assert.Contains(t, outEmpty, "No metrics for either job.")
}

func TestDashboardModel_UpdateConfirmationCoverage(t *testing.T) {
	m := NewDashboardModel("http://localhost")
	m.viewState = viewConfirmation

	actions := []string{
		"cancel multiple",
		"force complete multiple",
		"purge multiple",
		"pause group multiple",
		"resume group multiple",
		"retry multiple",
		"approve multiple",
		"hold multiple",
		"unhold multiple",
		"priority multiple",
	}

	m.jobs = []orchestrator.JobInfo{
		{ID: "JOB-1", WorkItem: orchestrator.WorkItem{Priority: 5, ConcurrencyGroup: "test-group"}},
	}
	for _, action := range actions {
		m.pendingAction = action
		m.selectedJobs = map[string]bool{"JOB-1": true}
		if action == "priority multiple" {
			m.pendingJobId = "MULTIPLE_up"
		} else {
			m.pendingJobId = "MULTIPLE"
		}
		newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		model := newM.(DashboardModel)
		assert.Equal(t, viewMain, model.viewState)
		assert.NotNil(t, cmd)
	}

	m.pendingAction = "priority multiple"
	m.pendingJobId = "MULTIPLE_down"
	m.selectedJobs = map[string]bool{"JOB-1": true}
	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := newM.(DashboardModel)
	assert.Equal(t, viewMain, model.viewState)
	assert.NotNil(t, cmd)

	singleActions := []string{
		"cancel",
		"force complete",
		"purge",
		"cancel all",
		"retry",
		"retry failed",
		"clear history",
		"clear pending",
		"hold",
		"unhold",
	}

	for _, action := range singleActions {
		m.viewState = viewConfirmation
		m.pendingAction = action
		m.pendingJobId = "JOB-1"
		newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		model := newM.(DashboardModel)
		assert.Equal(t, viewMain, model.viewState)
		assert.NotNil(t, cmd)
	}
}
