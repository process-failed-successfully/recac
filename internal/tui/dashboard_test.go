package tui

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"recac/internal/orchestrator"
	"strings"
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

func TestFetchStatus_Mocked(t *testing.T) {
	// Mock statusFetcher
	originalFetcher := statusFetcher
	defer func() { statusFetcher = originalFetcher }()

	statusFetcher = func(url string) (*http.Response, error) {
		if strings.Contains(url, "/status") {
			// Return status JSON
			body := `{"uptime": "1h", "active_spawns": 1}`
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}
		if strings.Contains(url, "/jobs") {
			// Return jobs JSON
			body := `[{"id": "job1", "status": "running"}]`
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}
		return nil, fmt.Errorf("unknown url")
	}

	cmd := fetchStatus("http://localhost")
	msg := cmd()

	sMsg, ok := msg.(statusMsg)
	assert.True(t, ok)
	assert.Nil(t, sMsg.Err)
	assert.Equal(t, "1h", sMsg.Status.Uptime)
	assert.Equal(t, 1, sMsg.Status.ActiveSpawns)
	assert.Len(t, sMsg.Jobs, 1)
	assert.Equal(t, "job1", sMsg.Jobs[0].ID)
}

func TestFetchStatus_Error_Mocked(t *testing.T) {
	originalFetcher := statusFetcher
	defer func() { statusFetcher = originalFetcher }()

	statusFetcher = func(url string) (*http.Response, error) {
		return nil, fmt.Errorf("network error")
	}

	cmd := fetchStatus("http://localhost")
	msg := cmd()

	sMsg, ok := msg.(statusMsg)
	assert.True(t, ok)
	assert.NotNil(t, sMsg.Err)
	assert.Equal(t, "network error", sMsg.Err.Error())
}

func TestStartDashboard(t *testing.T) {
	originalRunner := programRunner
	defer func() { programRunner = originalRunner }()

	programRunner = func(p *tea.Program) (tea.Model, error) {
		return nil, nil
	}

	err := StartDashboard("http://localhost")
	assert.NoError(t, err)
}
