package tui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"recac/internal/orchestrator"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestDashboard_ApproveJobKeybinding(t *testing.T) {
	mux := http.NewServeMux()

	approved := false
	mux.HandleFunc("/jobs/JOB-1/approve", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			approved = true
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		status := orchestrator.Status{}
		json.NewEncoder(w).Encode(status)
	})
	mux.HandleFunc("/jobs", func(w http.ResponseWriter, r *http.Request) {
		jobs := []orchestrator.JobInfo{
			{
				ID:        "JOB-1",
				Status:    "Pending Approval",
				StartTime: time.Now(),
				WorkItem:  orchestrator.WorkItem{Summary: "Test Approval"},
			},
		}
		json.NewEncoder(w).Encode(jobs)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	m := NewDashboardModel(server.URL)

	// Simulate receiving jobs
	model, _ := m.Update(statusMsg{
		Status: orchestrator.Status{},
		Jobs: []orchestrator.JobInfo{
			{
				ID:        "JOB-1",
				Status:    "Pending Approval",
				StartTime: time.Now(),
				WorkItem:  orchestrator.WorkItem{Summary: "Test Approval"},
			},
		},
	})

	m = model.(DashboardModel)

	// Select row in table manually if needed, or set it directly
	// Table has 1 row, by default it's selected. But let's be sure.
	rows := []table.Row{{"JOB-1", "Test Approval", "Pending Approval", "0s"}}
	m.table.SetRows(rows)
	m.table.SetCursor(0)

	// Press 'a'
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	assert.NotNil(t, cmd, "Expected a command from 'a' keybinding")

	// Execute command
	msg := cmd()

	// Should be an actionMsg with Approved string
	action, ok := msg.(actionMsg)
	assert.True(t, ok, "Expected actionMsg")
	assert.NoError(t, action.Err, "Expected no error from approve command")
	assert.Equal(t, "Approved", action.Message)
	assert.True(t, approved, "Expected API to be called")

	// Also check if 'a' is in the help view
	view := m.View()
	assert.Contains(t, view, "a: approve", "Expected 'a: approve' in help view")
}
