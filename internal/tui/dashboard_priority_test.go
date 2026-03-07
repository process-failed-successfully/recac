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

func TestDashboard_PriorityUpdateCmd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/jobs/TEST-123/priority", r.URL.Path)

		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"priority": 10}`)
	}))
	defer server.Close()

	cmd := updatePriorityCmd(server.URL, "TEST-123", 10)
	msg := cmd()

	actionMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.NoError(t, actionMsg.Err)
	assert.Contains(t, actionMsg.Message, "Updated priority for job TEST-123 to 10")
}

func TestDashboard_PriorityUpdateKeys(t *testing.T) {
	m := NewDashboardModel("http://dummy")

	// Setup a job and table
	m.jobs = []orchestrator.JobInfo{
		{
			ID: "TEST-123",
			WorkItem: orchestrator.WorkItem{
				Priority: 5,
			},
			StartTime: time.Now(),
		},
	}
	m.updateTableContent()
	m.table.SetCursor(0)

	// Test increment
	m2, cmdInc := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'>'}})
	m = m2.(DashboardModel)
	assert.NotNil(t, cmdInc)

	// Since we can't easily execute the cmd returned without a full tea Program,
	// we just ensure a command was returned for the `>` key.

	// Test decrement
	m3, cmdDec := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'<'}})
	m = m3.(DashboardModel)
	assert.NotNil(t, cmdDec)
}
