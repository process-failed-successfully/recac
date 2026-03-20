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
	assert.Equal(t, "[ ] JOB-2", rows[0][0]) // Sorted by time (newest first)
	assert.Equal(t, "[ ] JOB-1", rows[1][0])
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

	maxRetries := 3
	cmd := submitJobCmd(server.URL, "summary", "repo", "desc", []string{"JOB-1"}, []string{"tag1"}, "group-1", true, "custom-provider", "custom-model", &maxRetries)
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

	m.focusedInput = 9
	m.updateFocus()

	assert.False(t, m.inputs[2].Focused())
	assert.True(t, m.textarea.Focused())
}

func TestDashboardModel_ForceCompleteKeybind(t *testing.T) {
	// Setup a minimal model
	columns := []table.Column{
		{Title: "ID", Width: 10},
		{Title: "Summary", Width: 40},
		{Title: "Status", Width: 10},
		{Title: "Wait Time", Width: 15},
		{Title: "Run Time", Width: 15},
	}
	rows := []table.Row{
		{"JOB-1", "Test Job 1", "Running", "10s", "5s"},
	}
	tbl := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	m := DashboardModel{
		table:     tbl,
		viewState: viewMain,
		selectedJobs: map[string]bool{},
	}

	// Send 'F' key message
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'F'}}
	newModel, cmd := m.Update(msg)

	assert.Nil(t, cmd)

	updatedModel := newModel.(DashboardModel)
	assert.Equal(t, viewConfirmation, updatedModel.viewState)
	assert.Equal(t, "force complete", updatedModel.pendingAction)
	assert.Equal(t, "JOB-1", updatedModel.pendingJobId)
}

func TestDashboardModel_ForceCompleteMultipleKeybind(t *testing.T) {
	columns := []table.Column{
		{Title: "ID", Width: 10},
	}
	rows := []table.Row{
		{"JOB-1"},
		{"JOB-2"},
	}
	tbl := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	m := DashboardModel{
		table:     tbl,
		viewState: viewMain,
		selectedJobs: map[string]bool{
			"JOB-1": true,
			"JOB-2": true,
		},
	}

	// Send 'F' key message
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'F'}}
	newModel, cmd := m.Update(msg)

	assert.Nil(t, cmd)

	updatedModel := newModel.(DashboardModel)
	assert.Equal(t, viewConfirmation, updatedModel.viewState)
	assert.Equal(t, "force complete multiple", updatedModel.pendingAction)
	assert.Equal(t, "MULTIPLE_F", updatedModel.pendingJobId)
}

func TestDashboardModel_UpdateDepsInput(t *testing.T) {
	m := NewDashboardModel("http://localhost:2112")
	m.jobs = []orchestrator.JobInfo{
		{ID: "job-1", WorkItem: orchestrator.WorkItem{DependsOn: []string{"dep1"}}},
	}
	m.updateTableContent()
	m.table.SetCursor(0)

	// Send 'D' to trigger updateDepsInput view state
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D"), Alt: false})
	m = newModel.(DashboardModel)

	// Verify view state changes
	assert.Equal(t, viewDepsInput, m.viewState)
	assert.Equal(t, "job-1", m.pendingJobId)
	assert.Equal(t, "dep1", m.depsInput.Value())

	// Test Esc to blur and reset state
	newModelEsc, cmdEsc := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("esc"), Alt: false})
	mEsc := newModelEsc.(DashboardModel)
	assert.Nil(t, cmdEsc)
	assert.Equal(t, viewMain, mEsc.viewState)
	assert.Equal(t, "", mEsc.pendingJobId)

	// Update view state again
	newModel2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D"), Alt: false})
	m = newModel2.(DashboardModel)

	// Test Enter to confirm deps
	m.depsInput.SetValue("dep1, dep2")
	newModelEnter, cmdEnter := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("enter"), Alt: false})
	mEnter := newModelEnter.(DashboardModel)
	assert.NotNil(t, mEnter)

	assert.NotNil(t, cmdEnter) // Should return the updateDependenciesCmd
	assert.Equal(t, viewMain, mEnter.viewState)
	assert.Equal(t, "", mEnter.pendingJobId)
}

func TestDashboardModel_UpdateDepsInput_Multiple(t *testing.T) {
	m := NewDashboardModel("http://localhost:2112")
	m.jobs = []orchestrator.JobInfo{
		{ID: "job-1"},
		{ID: "job-2"},
	}
	m.selectedJobs = map[string]bool{"job-1": true, "job-2": true}
	m.viewState = viewDepsInput
	m.pendingJobId = "MULTIPLE_deps"
	m.depsInput.SetValue("dep1, dep2")

	newModelEnter, cmdEnter := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("enter"), Alt: false})
	mEnter := newModelEnter.(DashboardModel)
	assert.NotNil(t, mEnter)

	assert.NotNil(t, cmdEnter) // Should return tea.Batch
	assert.Equal(t, viewMain, mEnter.viewState)
	assert.Equal(t, "", mEnter.pendingJobId)
	assert.Empty(t, mEnter.selectedJobs)
}

func TestDashboardModel_FetchJob(t *testing.T) {
	jobInfo := orchestrator.JobInfo{
		ID:      "JOB-1",
		Summary: "Test Job",
	}

	server := setupMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs/JOB-1" && r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(jobInfo)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	})
	defer server.Close()

	// Success case
	job, err := fetchJob(server.URL, "JOB-1")
	assert.NoError(t, err)
	assert.NotNil(t, job)
	assert.Equal(t, "JOB-1", job.ID)
	assert.Equal(t, "Test Job", job.Summary)

	// Not Found case
	_, errNotFound := fetchJob(server.URL, "NON-EXISTENT-JOB")
	assert.Error(t, errNotFound)
	assert.Contains(t, errNotFound.Error(), "status 404")

	// Connection Error case
	_, errConn := fetchJob("http://invalid-url-that-does-not-exist:9999", "JOB-1")
	assert.Error(t, errConn)
	assert.Contains(t, errConn.Error(), "failed to connect to orchestrator")
}

func TestDashboardModel_ForceCompleteJobCmd(t *testing.T) {
	server := setupMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs/JOB-1/force-complete" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	})
	defer server.Close()

	// Success case
	cmd := forceCompleteJobCmd(server.URL, "JOB-1")
	msg := cmd()
	action, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Nil(t, action.Err)
	assert.Equal(t, "Force Completed", action.Message)

	// Connection Error case
	cmdErr := forceCompleteJobCmd("http://invalid-url-that-does-not-exist:9999", "JOB-1")
	msgErr := cmdErr()
	actionErr, okErr := msgErr.(actionMsg)
	assert.True(t, okErr)
	assert.NotNil(t, actionErr.Err)

	// Status Not OK case
	cmdNotFound := forceCompleteJobCmd(server.URL, "NON-EXISTENT-JOB")
	msgNotFound := cmdNotFound()
	actionNotFound, okNotFound := msgNotFound.(actionMsg)
	assert.True(t, okNotFound)
	assert.NotNil(t, actionNotFound.Err)
	assert.Contains(t, actionNotFound.Err.Error(), "status 404")
}

func TestStartDashboard2(t *testing.T) {
	m := NewDashboardModel("http://localhost:2112")
	assert.NotNil(t, m.table)
	assert.Equal(t, viewMain, m.viewState)
}

func TestDashboardModel_FetchCompareJobs(t *testing.T) {
	job1 := orchestrator.JobInfo{ID: "JOB-1"}
	job2 := orchestrator.JobInfo{ID: "JOB-2"}

	server := setupMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs/JOB-1" {
			json.NewEncoder(w).Encode(job1)
		} else if r.URL.Path == "/jobs/JOB-2" {
			json.NewEncoder(w).Encode(job2)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	})
	defer server.Close()

	cmd := fetchCompareJobs(server.URL, "JOB-1", "JOB-2")
	msg := cmd()
	compareMsgVal, ok := msg.(compareMsg)
	assert.True(t, ok)
	assert.Equal(t, "JOB-1", compareMsgVal.Jobs[0].ID)
	assert.Equal(t, "JOB-2", compareMsgVal.Jobs[1].ID)

	// Error case
	cmdErr := fetchCompareJobs("http://invalid-url:9999", "JOB-1", "JOB-2")
	msgErr := cmdErr()
	compareMsgErr, okErr := msgErr.(compareMsg)
	assert.True(t, okErr)
	assert.NotNil(t, compareMsgErr.Err)
}

func TestDashboardModel_UpdateAgentCmd(t *testing.T) {
	server := setupMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs/JOB-1/agent" && r.Method == http.MethodPut {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	})
	defer server.Close()

	// Success case
	cmd := updateAgentCmd(server.URL, "JOB-1", "provider", "model")
	msg := cmd()
	action, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Nil(t, action.Err)
	assert.Equal(t, "Updated agent for job JOB-1", action.Message)

	// Connection Error case
	cmdErr := updateAgentCmd("http://invalid-url-that-does-not-exist:9999", "JOB-1", "provider", "model")
	msgErr := cmdErr()
	actionErr, okErr := msgErr.(actionMsg)
	assert.True(t, okErr)
	assert.NotNil(t, actionErr.Err)
}

func TestDashboardModel_UpdateAgentInput(t *testing.T) {
	m := NewDashboardModel("http://localhost:2112")
	m.jobs = []orchestrator.JobInfo{
		{ID: "job-1"},
	}
	m.updateTableContent()
	m.table.SetCursor(0)

	// Switch to agent input view
	m.viewState = viewAgentInput
	m.pendingJobId = "job-1"

	// Focus first input
	m.agentProviderInput.Focus()
	m.agentProviderInput.SetValue("provider")

	// Esc test
	newModelEsc, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("esc"), Alt: false})
	mEsc := newModelEsc.(DashboardModel)
	assert.Equal(t, viewMain, mEsc.viewState)

	// Enter test when both have values
	m.agentModelInput.SetValue("model")
	newModelEnter, cmdEnter := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("enter"), Alt: false})
	mEnter := newModelEnter.(DashboardModel)
	assert.NotNil(t, mEnter)
	assert.Equal(t, viewMain, mEnter.viewState)
	assert.NotNil(t, cmdEnter)

	// Enter test for multiple
	m.selectedJobs = map[string]bool{"job-1": true, "job-2": true}
	m.viewState = viewAgentInput
	m.pendingJobId = "MULTIPLE_agent"
	newModelEnterMult, cmdEnterMult := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("enter"), Alt: false})
	mEnterMult := newModelEnterMult.(DashboardModel)
	assert.Equal(t, viewMain, mEnterMult.viewState)
	assert.NotNil(t, cmdEnterMult) // tea.Batch
	assert.Empty(t, mEnterMult.selectedJobs)

	// Tab test
	m.viewState = viewAgentInput
	m.agentProviderInput.Focus()
	newModelTab, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("tab"), Alt: false})
	mTab := newModelTab.(DashboardModel)
	assert.False(t, mTab.agentProviderInput.Focused())
	assert.True(t, mTab.agentModelInput.Focused())

	// Shift+Tab test
	newModelShiftTab, _ := mTab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("shift+tab"), Alt: false})
	mShiftTab := newModelShiftTab.(DashboardModel)
	assert.True(t, mShiftTab.agentProviderInput.Focused())
	assert.False(t, mShiftTab.agentModelInput.Focused())
}

func TestDashboardModel_UpdateEnvInput(t *testing.T) {
	m := NewDashboardModel("http://localhost:2112")
	m.jobs = []orchestrator.JobInfo{
		{ID: "job-1", WorkItem: orchestrator.WorkItem{EnvVars: map[string]string{"A": "B"}}},
	}
	m.updateTableContent()
	m.table.SetCursor(0)

	// Send 'E' to trigger updateEnvInput view state
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("E"), Alt: false})
	m = newModel.(DashboardModel)

	// Verify view state changes
	assert.Equal(t, viewEnvInput, m.viewState)
	assert.Equal(t, "job-1", m.pendingJobId)
	assert.Equal(t, "A=B", m.envInput.Value())

	// Test Esc to blur and reset state
	newModelEsc, cmdEsc := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("esc"), Alt: false})
	mEsc := newModelEsc.(DashboardModel)
	assert.Nil(t, cmdEsc)
	assert.Equal(t, viewMain, mEsc.viewState)
	assert.Equal(t, "", mEsc.pendingJobId)

	// Update view state again
	newModel2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("E"), Alt: false})
	m = newModel2.(DashboardModel)

	// Test Enter to confirm env
	m.envInput.SetValue("A=B, C=D")
	newModelEnter, cmdEnter := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("enter"), Alt: false})
	mEnter := newModelEnter.(DashboardModel)
	assert.NotNil(t, mEnter)

	assert.NotNil(t, cmdEnter) // Should return the updateEnvCmd
	assert.Equal(t, viewMain, mEnter.viewState)
	assert.Equal(t, "", mEnter.pendingJobId)
}

func TestDashboardModel_UpdateEnvInput_Multiple(t *testing.T) {
	m := NewDashboardModel("http://localhost:2112")
	m.jobs = []orchestrator.JobInfo{
		{ID: "job-1"},
		{ID: "job-2"},
	}
	m.selectedJobs = map[string]bool{"job-1": true, "job-2": true}
	m.viewState = viewEnvInput
	m.pendingJobId = "MULTIPLE_env"
	m.envInput.SetValue("A=B, C=D")

	newModelEnter, cmdEnter := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("enter"), Alt: false})
	mEnter := newModelEnter.(DashboardModel)
	assert.NotNil(t, mEnter)

	assert.NotNil(t, cmdEnter) // Should return tea.Batch
	assert.Equal(t, viewMain, mEnter.viewState)
	assert.Equal(t, "", mEnter.pendingJobId)
	assert.Empty(t, mEnter.selectedJobs)
}

func TestDashboardModel_UpdateTagsInput(t *testing.T) {
	m := NewDashboardModel("http://localhost:2112")
	m.jobs = []orchestrator.JobInfo{
		{ID: "job-1", WorkItem: orchestrator.WorkItem{Tags: []string{"tag1"}}},
	}
	m.updateTableContent()
	m.table.SetCursor(0)

	// Send 'G' to trigger updateTagsInput view state
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G"), Alt: false})
	m = newModel.(DashboardModel)

	// Verify view state changes
	assert.Equal(t, viewTagsInput, m.viewState)
	assert.Equal(t, "job-1", m.pendingJobId)
	assert.Equal(t, "tag1", m.tagsInput.Value())

	// Test Esc to blur and reset state
	newModelEsc, cmdEsc := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("esc"), Alt: false})
	mEsc := newModelEsc.(DashboardModel)
	assert.Nil(t, cmdEsc)
	assert.Equal(t, viewMain, mEsc.viewState)
	assert.Equal(t, "", mEsc.pendingJobId)

	// Update view state again
	newModel2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G"), Alt: false})
	m = newModel2.(DashboardModel)

	// Test Enter to confirm tags
	m.tagsInput.SetValue("tag1, tag2")
	newModelEnter, cmdEnter := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("enter"), Alt: false})
	mEnter := newModelEnter.(DashboardModel)
	assert.NotNil(t, mEnter)

	assert.NotNil(t, cmdEnter) // Should return the updateTagsCmd
	assert.Equal(t, viewMain, mEnter.viewState)
	assert.Equal(t, "", mEnter.pendingJobId)
}

func TestDashboardModel_UpdateTagsInput_Multiple(t *testing.T) {
	m := NewDashboardModel("http://localhost:2112")
	m.jobs = []orchestrator.JobInfo{
		{ID: "job-1"},
		{ID: "job-2"},
	}
	m.selectedJobs = map[string]bool{"job-1": true, "job-2": true}
	m.viewState = viewTagsInput
	m.pendingJobId = "MULTIPLE_tags"
	m.tagsInput.SetValue("tag1, tag2")

	newModelEnter, cmdEnter := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("enter"), Alt: false})
	mEnter := newModelEnter.(DashboardModel)
	assert.NotNil(t, mEnter)

	assert.NotNil(t, cmdEnter) // Should return tea.Batch
	assert.Equal(t, viewMain, mEnter.viewState)
	assert.Equal(t, "", mEnter.pendingJobId)
	assert.Empty(t, mEnter.selectedJobs)
}

func TestDashboardModel_UpdateRenameInput(t *testing.T) {
	m := NewDashboardModel("http://localhost:2112")
	m.jobs = []orchestrator.JobInfo{
		{ID: "job-1"},
	}
	m.updateTableContent()
	m.table.SetCursor(0)

	// Send 'N' to trigger updateRenameInput view state
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("N"), Alt: false})
	m = newModel.(DashboardModel)

	// Verify view state changes
	assert.Equal(t, viewRenameInput, m.viewState)
	assert.Equal(t, "job-1", m.pendingJobId)
	assert.Equal(t, "job-1", m.renameInput.Value())

	// Test Esc to blur and reset state
	newModelEsc, cmdEsc := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("esc"), Alt: false})
	mEsc := newModelEsc.(DashboardModel)
	assert.Nil(t, cmdEsc)
	assert.Equal(t, viewMain, mEsc.viewState)
	assert.Equal(t, "", mEsc.pendingJobId)

	// Update view state again
	newModel2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("N"), Alt: false})
	m = newModel2.(DashboardModel)

	// Test Enter to confirm rename
	m.renameInput.SetValue("job-2")
	newModelEnter, cmdEnter := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("enter"), Alt: false})
	mEnter := newModelEnter.(DashboardModel)
	assert.NotNil(t, mEnter)

	assert.NotNil(t, cmdEnter) // Should return the updateRenameCmd
	assert.Equal(t, viewMain, mEnter.viewState)
	assert.Equal(t, "", mEnter.pendingJobId)
}

func TestDashboardModel_ViewStates(t *testing.T) {
	m := DashboardModel{
		host: "host",
		status: orchestrator.Status{
			Uptime: "1h",
		},
	}

	m.viewState = viewCompare
	view := m.View()
	assert.Contains(t, view, "esc/q: back")

	m.viewState = viewAnalytics
	view = m.View()
	assert.Contains(t, view, "esc/q: back")

	m.viewState = viewTree
	view = m.View()
	assert.Contains(t, view, "esc/q: back")

	m.viewState = viewSearchLogsInput
	view = m.View()
	assert.Contains(t, view, "Search Logs (Regex):")

	m.viewState = viewSearchLogsResult
	view = m.View()
	assert.Contains(t, view, "esc/q: back")

	m.viewState = viewExplain
	view = m.View()
	assert.Contains(t, view, "esc/q: back")
}

func TestDashboardModel_UpdateSearchLogsInput(t *testing.T) {
	m := NewDashboardModel("http://localhost:2112")

	// Switch to search logs input view
	m.viewState = viewSearchLogsInput

	// Focus first input
	m.searchInput.Focus()
	m.searchInput.SetValue("error")

	// Esc test
	newModelEsc, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("esc"), Alt: false})
	mEsc := newModelEsc.(DashboardModel)
	assert.Equal(t, viewMain, mEsc.viewState)

	// Enter test
	newModelEnter, cmdEnter := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("enter"), Alt: false})
	mEnter := newModelEnter.(DashboardModel)
	assert.NotNil(t, mEnter)
	assert.NotNil(t, cmdEnter) // Should return searchLogsCmd
}

func TestDashboardModel_ToggleDrain(t *testing.T) {
	server := setupMockServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	cmd := toggleDrain(server.URL, false)
	msg := cmd()
	action, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Equal(t, "Draining", action.Message)

	cmd = toggleDrain(server.URL, true)
	msg = cmd()
	action, ok = msg.(actionMsg)
	assert.True(t, ok)
	assert.Equal(t, "Undrained", action.Message)
}
func TestDashboardModel_UpdateSearchLogsResult(t *testing.T) {
	m := NewDashboardModel("http://localhost:2112")
	m.viewState = viewSearchLogsInput
	m.searchInput.Focus()
	m.searchInput.SetValue("error")
	newModelEnter, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("enter"), Alt: false})
	mEnter := newModelEnter.(DashboardModel)
	assert.NotNil(t, mEnter)
	assert.Equal(t, viewSearchLogsInput, mEnter.viewState)
}
