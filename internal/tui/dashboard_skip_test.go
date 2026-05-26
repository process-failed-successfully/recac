package tui

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"recac/internal/orchestrator"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestDashboardModel_SkipFlow(t *testing.T) {
	var skipCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs/JOB-1/skip" && r.Method == http.MethodPost {
			skipCalled = true
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	m := NewDashboardModel(server.URL)
	m.jobs = []orchestrator.JobInfo{{ID: "JOB-1", Status: "Pending"}}
	m.updateTableContent()
	m.table.SetCursor(0)

	// User presses "I" key
	modelAfterKey, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'I'}})
	m = modelAfterKey.(DashboardModel)

	assert.Equal(t, "skip", m.pendingAction)
	assert.Equal(t, viewConfirmation, m.viewState)

	// User confirms
	modelAfterConfirm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = modelAfterConfirm.(DashboardModel)
	assert.Equal(t, viewMain, m.viewState)
	assert.NotNil(t, cmd)

	msg := cmd()
	action, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.NoError(t, action.Err)
	assert.True(t, skipCalled)
}

func TestDashboardModel_SkipDownstreamFlow(t *testing.T) {
	var skipCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs/JOB-1/skip" && r.URL.Query().Get("downstream") == "true" && r.Method == http.MethodPost {
			skipCalled = true
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	m := NewDashboardModel(server.URL)
	m.jobs = []orchestrator.JobInfo{{ID: "JOB-1", Status: "Pending"}}
	m.updateTableContent()
	m.table.SetCursor(0)

	// User presses "ctrl+w" key
	modelAfterKey, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("ctrl+w")})
	m = modelAfterKey.(DashboardModel)

	assert.Equal(t, "skip downstream", m.pendingAction)
	assert.Equal(t, viewConfirmation, m.viewState)

	// User confirms
	modelAfterConfirm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = modelAfterConfirm.(DashboardModel)
	assert.Equal(t, viewMain, m.viewState)
	assert.NotNil(t, cmd)

	msg := cmd()
	action, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.NoError(t, action.Err)
	assert.True(t, skipCalled)
}

func TestDashboardModel_SkipDownstreamMultipleFlow(t *testing.T) {
	var skipCalledCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if (r.URL.Path == "/jobs/JOB-1/skip" || r.URL.Path == "/jobs/JOB-2/skip") && r.URL.Query().Get("downstream") == "true" && r.Method == http.MethodPost {
			skipCalledCount++
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	m := NewDashboardModel(server.URL)
	m.jobs = []orchestrator.JobInfo{
		{ID: "JOB-1", Status: "Pending"},
		{ID: "JOB-2", Status: "Pending"},
	}
	m.selectedJobs = map[string]bool{"JOB-1": true, "JOB-2": true}
	m.updateTableContent()

	// User presses "ctrl+w" key
	modelAfterKey, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("ctrl+w")})
	m = modelAfterKey.(DashboardModel)

	assert.Equal(t, "skip downstream multiple", m.pendingAction)
	assert.Equal(t, viewConfirmation, m.viewState)

	// User confirms
	modelAfterConfirm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = modelAfterConfirm.(DashboardModel)
	assert.Equal(t, viewMain, m.viewState)
	assert.NotNil(t, cmd)

	// Process batch command
	batchMsg := cmd()
	batch, ok := batchMsg.(tea.BatchMsg)
	assert.True(t, ok)
	assert.Equal(t, 2, len(batch))

	for _, c := range batch {
		msg := c()
		action, ok := msg.(actionMsg)
		assert.True(t, ok)
		assert.NoError(t, action.Err)
	}
	assert.Equal(t, 2, skipCalledCount)
}

func TestDashboardModel_SkipMultipleFlow(t *testing.T) {
	var skipCalledCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if (r.URL.Path == "/jobs/JOB-1/skip" || r.URL.Path == "/jobs/JOB-2/skip") && r.Method == http.MethodPost {
			skipCalledCount++
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	m := NewDashboardModel(server.URL)
	m.jobs = []orchestrator.JobInfo{
		{ID: "JOB-1", Status: "Pending"},
		{ID: "JOB-2", Status: "Pending"},
	}
	m.selectedJobs = map[string]bool{"JOB-1": true, "JOB-2": true}
	m.updateTableContent()

	// User presses "I" key
	modelAfterKey, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'I'}})
	m = modelAfterKey.(DashboardModel)

	assert.Equal(t, "skip multiple", m.pendingAction)
	assert.Equal(t, viewConfirmation, m.viewState)

	// User confirms
	modelAfterConfirm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = modelAfterConfirm.(DashboardModel)
	assert.Equal(t, viewMain, m.viewState)
	assert.NotNil(t, cmd)

	// Process batch command
	batchMsg := cmd()
	batch, ok := batchMsg.(tea.BatchMsg)
	assert.True(t, ok)
	assert.Equal(t, 2, len(batch))

	for _, c := range batch {
		msg := c()
		action, ok := msg.(actionMsg)
		assert.True(t, ok)
		assert.NoError(t, action.Err)
	}
	assert.Equal(t, 2, skipCalledCount)
}
