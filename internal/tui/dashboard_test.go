package tui

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"recac/internal/orchestrator"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestDashboardModel_Update_StatusMsg(t *testing.T) {
	// Setup
	columns := []table.Column{
		{Title: "ID", Width: 10},
		{Title: "Summary", Width: 40},
		{Title: "Status", Width: 10},
		{Title: "Duration", Width: 10},
	}
	tModel := table.New(table.WithColumns(columns))

	model := DashboardModel{
		host:  "http://localhost",
		table: tModel,
	}

	// Create mock data
	status := orchestrator.Status{
		Uptime:       "1h",
		PollInterval: "1m",
		ActiveSpawns: 5,
	}

	now := time.Now()
	jobs := []orchestrator.JobInfo{
		{ID: "JOB-1", Summary: "Test Job 1", Status: "Running", StartTime: now.Add(-10 * time.Minute)},
		{ID: "JOB-2", Summary: "Test Job 2", Status: "Pending", StartTime: now.Add(-1 * time.Minute)},
	}

	msg := statusMsg{
		Status: status,
		Jobs:   jobs,
		Err:    nil,
	}

	// Act
	updatedModel, cmd := model.Update(msg)

	// Assert
	m, ok := updatedModel.(DashboardModel)
	assert.True(t, ok)

	assert.Equal(t, status, m.status)
	assert.Equal(t, jobs, m.jobs)
	assert.Nil(t, m.err)
	assert.Nil(t, cmd) // No command returned on status update (except table update which returns nil usually)

	// Check table rows
	rows := m.table.Rows()
	assert.Len(t, rows, 2)
	assert.Equal(t, "JOB-2", rows[0][0]) // Sorted by time (newest first)
	assert.Equal(t, "JOB-1", rows[1][0])
}

func TestDashboardModel_Update_Error(t *testing.T) {
	model := DashboardModel{}
	err := errors.New("network error")
	msg := statusMsg{Err: err}

	updatedModel, _ := model.Update(msg)
	m := updatedModel.(DashboardModel)

	assert.Equal(t, err, m.err)
}

func TestDashboardModel_View(t *testing.T) {
	columns := []table.Column{
		{Title: "ID", Width: 10},
		{Title: "Summary", Width: 40},
		{Title: "Status", Width: 10},
		{Title: "Duration", Width: 10},
	}
	tModel := table.New(table.WithColumns(columns))
	model := DashboardModel{
		host:  "test-host",
		table: tModel,
		status: orchestrator.Status{
			Uptime: "10m",
		},
	}

	// Add a job so it's not empty state
	model.jobs = []orchestrator.JobInfo{{ID: "JOB-1"}}

	view := model.View()
	assert.Contains(t, view, "Orchestrator Dashboard")
	assert.Contains(t, view, "Host: test-host")
	assert.Contains(t, view, "Uptime: 10m")
}

func TestDashboardModel_Quit(t *testing.T) {
	model := DashboardModel{}
	updatedModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	m := updatedModel.(DashboardModel)

	assert.True(t, m.quitting)
	assert.NotNil(t, cmd) // Should return tea.Quit
}

func TestDashboardModel_View_EmptyState(t *testing.T) {
	// Setup
	columns := []table.Column{
		{Title: "ID", Width: 10},
		{Title: "Summary", Width: 40},
		{Title: "Status", Width: 10},
		{Title: "Duration", Width: 10},
	}
	tModel := table.New(table.WithColumns(columns))

	model := DashboardModel{
		host:  "test-host",
		table: tModel,
		jobs:  []orchestrator.JobInfo{}, // Empty jobs
		status: orchestrator.Status{
			Uptime: "10m",
		},
		viewState: viewMain,
	}

	// Act
	view := model.View()

	// Assert
	assert.Contains(t, view, "No active jobs found")
}

func setupMockServer(handler http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(handler)
}

func TestDashboardModel_TogglePause(t *testing.T) {
	server := setupMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/pause" {
			w.WriteHeader(http.StatusOK)
		} else if r.URL.Path == "/resume" {
			w.WriteHeader(http.StatusOK)
		}
	})
	defer server.Close()

	cmd := togglePause(server.URL, false)
	msg := cmd()
	action, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Equal(t, "Paused", action.Message)

	cmd = togglePause(server.URL, true)
	msg = cmd()
	action, ok = msg.(actionMsg)
	assert.True(t, ok)
	assert.Equal(t, "Resumed", action.Message)
}

func TestDashboardModel_ForcePoll(t *testing.T) {
	server := setupMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/poll" {
			w.WriteHeader(http.StatusOK)
		}
	})
	defer server.Close()

	cmd := forcePoll(server.URL)
	msg := cmd()
	action, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Equal(t, "Poll triggered", action.Message)
}

func TestDashboardModel_ClearHistory(t *testing.T) {
	server := setupMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/history" && r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"cleared": 5})
		}
	})
	defer server.Close()

	cmd := clearHistory(server.URL)
	msg := cmd()
	action, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Equal(t, "Cleared 5 jobs", action.Message)
}

func TestDashboardModel_SubmitJob(t *testing.T) {
	server := setupMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusAccepted)
		}
	})
	defer server.Close()

	cmd := submitJobCmd(server.URL, "summary", "repo", "desc", []string{"JOB-1"})
	msg := cmd()
	action, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Equal(t, "Job submitted successfully", action.Message)
}

func TestDashboardModel_CancelAllJobs(t *testing.T) {
	server := setupMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs" && r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"canceled": 2})
		}
	})
	defer server.Close()

	cmd := cancelAllJobs(server.URL)
	msg := cmd()
	action, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Equal(t, "Cancelled 2 Jobs", action.Message)
}

func TestDashboardModel_RetryFailedJobs(t *testing.T) {
	server := setupMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs/retry-failed" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"retried": 3})
		}
	})
	defer server.Close()

	cmd := retryFailedJobs(server.URL)
	msg := cmd()
	action, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Equal(t, "Retried 3 failed jobs", action.Message)
}

func TestDashboardModel_CancelJob(t *testing.T) {
	server := setupMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs/123" && r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusOK)
		}
	})
	defer server.Close()

	cmd := cancelJob(server.URL, "123")
	msg := cmd()
	action, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Equal(t, "Cancelled", action.Message)
}

func TestDashboardModel_RetryJob(t *testing.T) {
	server := setupMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs/123/retry" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusAccepted)
		}
	})
	defer server.Close()

	cmd := retryJob(server.URL, "123")
	msg := cmd()
	action, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Equal(t, "Retried", action.Message)
}

func TestDashboardModel_OpenBrowser(t *testing.T) {
	oldOpenBrowser := utilsOpenBrowser
	utilsOpenBrowser = func(url string) error {
		return nil
	}
	defer func() { utilsOpenBrowser = oldOpenBrowser }()

	cmd := openBrowserCmd("http://example.com")
	msg := cmd()
	action, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Equal(t, "Opened browser", action.Message)
}

func TestDashboardModel_Tick(t *testing.T) {
	cmd := tick()
	msg := cmd()
	_, ok := msg.(tickMsg)
	assert.True(t, ok)
}

func TestStartDashboard(t *testing.T) {
	// similar to explorer
	m := NewDashboardModel("http://localhost")
	assert.NotNil(t, m.table)
}

func TestDashboardModel_UpdateFocus(t *testing.T) {
	m := NewDashboardModel("http://localhost")

	// initially input 0 is focused
	assert.True(t, m.inputs[0].Focused())
	assert.False(t, m.inputs[1].Focused())
	assert.False(t, m.textarea.Focused())

	m.focusedInput = 1
	m.updateFocus()

	assert.False(t, m.inputs[0].Focused())
	assert.True(t, m.inputs[1].Focused())

	m.focusedInput = 2
	m.updateFocus()

	assert.False(t, m.inputs[1].Focused())
	assert.True(t, m.inputs[2].Focused())

	m.focusedInput = 3
	m.updateFocus()

	assert.False(t, m.inputs[2].Focused())
	assert.True(t, m.textarea.Focused())
}
