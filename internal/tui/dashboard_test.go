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
	tModel := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
	)

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
	updatedModel, _ := model.Update(msg)
	m, ok := updatedModel.(DashboardModel)
	assert.True(t, ok)

	assert.Equal(t, status, m.status)
	assert.Equal(t, jobs, m.jobs)
	assert.Nil(t, m.err)
	// cmd can be nil or not depending on implementation, here table update returns nil usually

	// Check table rows
	rows := m.table.Rows()
	assert.Len(t, rows, 2)
	// Note: The implementation sorts jobs by StartTime descending (newest first).
	// JOB-2 started 1 min ago (newer), JOB-1 started 10 mins ago.
	// So JOB-2 should be first.
	if len(rows) == 2 {
		assert.Equal(t, "JOB-2", rows[0][0])
		assert.Equal(t, "JOB-1", rows[1][0])
	}
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

	view := model.View()
	assert.Contains(t, view, "Orchestrator Dashboard")
	assert.Contains(t, view, "Host: test-host")
	assert.Contains(t, view, "Uptime: 10m")
}

func TestDashboardModel_Quit(t *testing.T) {
	model := DashboardModel{}
	updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	m := updatedModel.(DashboardModel)

	assert.True(t, m.quitting)
	// cmd should be tea.Quit, but hard to equality check functions
}

func TestFetchStatus(t *testing.T) {
	// Mock server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/status" {
			json.NewEncoder(w).Encode(orchestrator.Status{ActiveSpawns: 5})
		} else if r.URL.Path == "/jobs" {
			json.NewEncoder(w).Encode([]orchestrator.JobInfo{{ID: "job1"}})
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	cmd := fetchStatus(ts.URL, false)
	msg := cmd()

	sMsg, ok := msg.(statusMsg)
	if !ok {
		t.Fatalf("Expected statusMsg, got %T", msg)
	}

	if sMsg.Err != nil {
		t.Fatalf("Unexpected error: %v", sMsg.Err)
	}

	if sMsg.Status.ActiveSpawns != 5 {
		t.Errorf("Expected 5 active spawns, got %d", sMsg.Status.ActiveSpawns)
	}
	if len(sMsg.Jobs) != 1 || sMsg.Jobs[0].ID != "job1" {
		t.Errorf("Expected job1, got %v", sMsg.Jobs)
	}
}

func TestFetchJobDetails(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs/job1" {
			json.NewEncoder(w).Encode(orchestrator.JobInfo{ID: "job1", Summary: "Details"})
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	cmd := fetchJobDetails(ts.URL, "job1")
	msg := cmd()

	dMsg, ok := msg.(detailsMsg)
	if !ok {
		t.Fatalf("Expected detailsMsg, got %T", msg)
	}

	if dMsg.Err != nil {
		t.Fatalf("Unexpected error: %v", dMsg.Err)
	}

	if dMsg.Job.Summary != "Details" {
		t.Errorf("Expected summary 'Details', got '%s'", dMsg.Job.Summary)
	}
}

func TestFetchJobDetails_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	cmd := fetchJobDetails(ts.URL, "job1")
	msg := cmd()

	dMsg, ok := msg.(detailsMsg)
	if !ok {
		t.Fatalf("Expected detailsMsg, got %T", msg)
	}

	if dMsg.Err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestCancelJob(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" && r.URL.Path == "/jobs/job1" {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer ts.Close()

	cmd := cancelJob(ts.URL, "job1")
	msg := cmd()

	aMsg, ok := msg.(actionMsg)
	if !ok {
		t.Fatalf("Expected actionMsg, got %T", msg)
	}
	if aMsg.Err != nil {
		t.Errorf("Unexpected error: %v", aMsg.Err)
	}
	if aMsg.Message != "Cancelled" {
		t.Errorf("Expected 'Cancelled', got '%s'", aMsg.Message)
	}
}

func TestRetryJob(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/jobs/job1/retry" {
			w.WriteHeader(http.StatusAccepted)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer ts.Close()

	cmd := retryJob(ts.URL, "job1")
	msg := cmd()

	aMsg, ok := msg.(actionMsg)
	if !ok {
		t.Fatalf("Expected actionMsg, got %T", msg)
	}
	if aMsg.Err != nil {
		t.Errorf("Unexpected error: %v", aMsg.Err)
	}
	if aMsg.Message != "Retried" {
		t.Errorf("Expected 'Retried', got '%s'", aMsg.Message)
	}
}

func TestFetchStatus_JSONDecodeError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/status" {
			w.Write([]byte("invalid json"))
		}
	}))
	defer ts.Close()

	cmd := fetchStatus(ts.URL, false)
	msg := cmd()
	sMsg, ok := msg.(statusMsg)
	if !ok {
		t.Fatalf("Expected statusMsg")
	}
	if sMsg.Err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestFetchStatus_JobsRequestError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/status" {
			json.NewEncoder(w).Encode(orchestrator.Status{})
		} else if r.URL.Path == "/jobs" {
			// Close connection or error
			// Simulating error for second request is tricky with httptest if we want the first to succeed.
			// But we can return 500 or just invalid JSON, but http.Get error is network error.
			// If we return 500, http.Get returns success (200 OK is not checked in fetchStatus for jobs? Let's check code).
			// http.Get returns err only on network error.
			// fetchStatus does NOT check status code for jobs request!
			// "jResp, err := http.Get(url); if err != nil ..."
			// So if server returns 500, err is nil.
			// Then json decode tries to decode body.
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer ts.Close()

	cmd := fetchStatus(ts.URL, false)
	msg := cmd()
	sMsg, ok := msg.(statusMsg)
	if !ok {
		t.Fatalf("Expected statusMsg")
	}
	// It should fail at JSON decode if 500 body is not valid JSON or empty.
	// If 500 body is empty, Decode returns EOF.
	if sMsg.Err == nil {
		t.Error("Expected error for jobs failure")
	}
}
