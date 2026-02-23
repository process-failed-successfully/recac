package tui

import (
	"errors"
	"recac/internal/orchestrator"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

// Helper to create a model with rows for coverage tests
func createCoverageTestModel() DashboardModel {
	m := NewDashboardModel("host")
	m.jobs = []orchestrator.JobInfo{
		{ID: "job1", Summary: "test", Status: "running", StartTime: time.Now()},
	}
	m.updateTableContent()
	return m
}

func TestDashboardModel_Update_DetailsMsg_Error_Coverage(t *testing.T) {
	m := createCoverageTestModel()
	err := errors.New("fail")
	msg := detailsMsg{Err: err}

	updated, _ := m.Update(msg)
	newM := updated.(DashboardModel)

	assert.Equal(t, err, newM.err)
}

func TestDashboardModel_Update_ActionMsg_Coverage(t *testing.T) {
	m := createCoverageTestModel()

	msg := actionMsg{Message: "Done"}
	updated, cmd := m.Update(msg)
	newM := updated.(DashboardModel)

	assert.Nil(t, newM.err)
	assert.NotNil(t, cmd) // Should fetch status
}

func TestDashboardModel_KeyMsg_L_Coverage(t *testing.T) {
	m := createCoverageTestModel()
	m.table.SetCursor(0)

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")}
	updated, cmd := m.Update(msg)
	// cmd should be streamJobLogs
	assert.NotNil(t, cmd)
	_ = updated
}

func TestDashboardModel_KeyMsg_C_Coverage(t *testing.T) {
	m := createCoverageTestModel()
	m.table.SetCursor(0)

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")}
	updated, cmd := m.Update(msg)
	// cmd should be cancelJob
	assert.NotNil(t, cmd)
	_ = updated
}

func TestDashboardModel_KeyMsg_R_Coverage(t *testing.T) {
	m := createCoverageTestModel()
	m.table.SetCursor(0)

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")}
	updated, cmd := m.Update(msg)
	// cmd should be retryJob
	assert.NotNil(t, cmd)
	_ = updated
}

func TestRenderDetails_Coverage(t *testing.T) {
	job := orchestrator.JobInfo{
		ID:        "job1",
		Summary:   "summary",
		Status:    "running",
		StartTime: time.Now(),
		WorkItem: orchestrator.WorkItem{
			RepoURL:     "http://repo",
			Description: "desc",
			EnvVars:     map[string]string{"FOO": "BAR"},
		},
	}

	details := renderDetails(job)
	assert.Contains(t, details, "job1")
	assert.Contains(t, details, "summary")
	assert.Contains(t, details, "running")
	assert.Contains(t, details, "FOO=BAR")

	// Test with EndTime
	job.EndTime = time.Now().Add(1 * time.Minute)
	details = renderDetails(job)
	assert.Contains(t, details, "End Time")

	// Test with Error
	job.Error = "something failed"
	details = renderDetails(job)
	assert.Contains(t, details, "something failed")
}

func TestLimitString_Coverage(t *testing.T) {
	s := "hello world"
	assert.Equal(t, "hello...", limitString(s, 5))
	assert.Equal(t, "hello world", limitString(s, 20))
}
