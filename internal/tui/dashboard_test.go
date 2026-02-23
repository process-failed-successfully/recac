package tui

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"recac/internal/orchestrator"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
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
		host: "test-host",
		table: tModel,
		status: orchestrator.Status{
			Uptime: "10m",
		},
	}

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

func TestDashboardModel_Update_DetailsMsg(t *testing.T) {
	model := DashboardModel{
		viewport: viewport.New(100, 20),
	}
	job := orchestrator.JobInfo{
		ID: "JOB-1", Summary: "Summary", Status: "Running",
		WorkItem: orchestrator.WorkItem{Description: "Desc", RepoURL: "http://repo"},
	}
	msg := detailsMsg{Job: job}

	updated, _ := model.Update(msg)
	m := updated.(DashboardModel)

	assert.Equal(t, job, m.details)
	assert.Equal(t, viewDetails, m.viewState)
	assert.Contains(t, m.viewport.View(), "JOB-1")
}

func TestDashboardModel_Update_LogStreamMsg(t *testing.T) {
	model := DashboardModel{
		viewport: viewport.New(100, 20),
	}
	stream := io.NopCloser(strings.NewReader("log content"))
	msg := logStreamMsg{Stream: stream}

	updated, cmd := model.Update(msg)
	m := updated.(DashboardModel)

	assert.Equal(t, viewLogs, m.viewState)
	assert.Equal(t, stream, m.logStream)
	assert.NotNil(t, cmd) // Should return waitForLogChunk command
}

func TestDashboardModel_Update_LogChunkMsg(t *testing.T) {
	model := DashboardModel{
		viewport:  viewport.New(100, 20),
		logStream: io.NopCloser(strings.NewReader("")),
		viewState: viewLogs,
	}
	msg := logChunkMsg{Chunk: "chunk1"}

	updated, cmd := model.Update(msg)
	m := updated.(DashboardModel)

	assert.Equal(t, "chunk1", m.logs)
	assert.Contains(t, m.viewport.View(), "chunk1")
	assert.NotNil(t, cmd) // Should return next waitForLogChunk command
}

func TestDashboardCommands(t *testing.T) {
	// Setup mock server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/status":
			w.Write([]byte(`{"uptime":"1h"}`))
		case "/jobs":
			w.Write([]byte(`[{"id":"1"}]`))
		case "/jobs/1":
			if r.Method == "DELETE" {
				w.WriteHeader(http.StatusOK)
			} else {
				w.Write([]byte(`{"id":"1", "summary":"test"}`))
			}
		case "/jobs/1/logs":
			w.Write([]byte("log data"))
		case "/jobs/1/retry":
			w.WriteHeader(http.StatusAccepted)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	// Test fetchStatus
	cmd := fetchStatus(ts.URL, false)
	msg := cmd()
	assert.IsType(t, statusMsg{}, msg)
	sMsg := msg.(statusMsg)
	assert.Nil(t, sMsg.Err)
	assert.Equal(t, "1h", sMsg.Status.Uptime)

	// Test fetchJobDetails
	cmd = fetchJobDetails(ts.URL, "1")
	msg = cmd()
	assert.IsType(t, detailsMsg{}, msg)
	dMsg := msg.(detailsMsg)
	assert.Nil(t, dMsg.Err)
	assert.Equal(t, "1", dMsg.Job.ID)

	// Test streamJobLogs
	cmd = streamJobLogs(ts.URL, "1")
	msg = cmd()
	assert.IsType(t, logStreamMsg{}, msg)
	lMsg := msg.(logStreamMsg)
	assert.Nil(t, lMsg.Err)
	content, _ := io.ReadAll(lMsg.Stream)
	assert.Equal(t, "log data", string(content))

	// Test cancelJob
	cmd = cancelJob(ts.URL, "1")
	msg = cmd()
	assert.IsType(t, actionMsg{}, msg)
	aMsg := msg.(actionMsg)
	assert.Nil(t, aMsg.Err)
	assert.Equal(t, "Cancelled", aMsg.Message)

	// Test retryJob
	cmd = retryJob(ts.URL, "1")
	msg = cmd()
	assert.IsType(t, actionMsg{}, msg)
	rMsg := msg.(actionMsg)
	assert.Nil(t, rMsg.Err)
	assert.Equal(t, "Retried", rMsg.Message)
}

func TestNewDashboardModel(t *testing.T) {
	m := NewDashboardModel("host")
	assert.Equal(t, "host", m.host)
	assert.NotNil(t, m.table)
	assert.NotNil(t, m.viewport)
	assert.Equal(t, viewMain, m.viewState)
}

func TestDashboardModel_Update_Keys(t *testing.T) {
	model := NewDashboardModel("host")

	// Test "l" (logs) without selection (should do nothing)
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	m := updated.(DashboardModel)
	assert.Equal(t, viewMain, m.viewState)

	// Test "enter" without selection
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(DashboardModel)
	assert.Equal(t, viewMain, m.viewState)

	// Test "h" (history)
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	m = updated.(DashboardModel)
	assert.True(t, m.showHistory)
	assert.NotNil(t, cmd)

	// Test viewport keys (esc)
	model.viewState = viewLogs
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(DashboardModel)
	assert.Equal(t, viewMain, m.viewState)
}
