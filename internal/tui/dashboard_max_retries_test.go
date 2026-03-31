package tui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"recac/internal/orchestrator"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestUpdateMaxRetriesCmd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/jobs/JOB-1/max-retries", r.URL.Path)
		var body map[string]int
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, 5, body["max_retries"])
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cmd := updateMaxRetriesCmd(server.URL, "JOB-1", 5)
	msg := cmd()

	actionMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.NoError(t, actionMsg.Err)
	assert.Equal(t, "Updated max retries for job JOB-1 to 5", actionMsg.Message)
}

func TestDashboardModel_UpdateMaxRetries(t *testing.T) {
	m := NewDashboardModel("http://localhost:2112")
	m.jobs = []orchestrator.JobInfo{
		{
			ID:      "TEST-123",
			Summary: "Test Job",
			Status:  "Pending",
			WorkItem: orchestrator.WorkItem{
				MaxRetries: func(i int) *int { return &i }(3),
			},
		},
	}
	m.updateTableContent()
	m.table.SetCursor(0)

	t.Run("Open Max Retries Input", func(t *testing.T) {
		newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Z")})
		model := newM.(DashboardModel)
		assert.Equal(t, viewMaxRetriesInput, model.viewState)
		assert.Equal(t, "TEST-123", model.pendingJobId)
		assert.Equal(t, "3", model.maxRetriesInput.Value())
	})

	t.Run("Open Max Retries Input - Multiple Selection", func(t *testing.T) {
		m.selectedJobs["TEST-123"] = true
		newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Z")})
		model := newM.(DashboardModel)
		assert.Equal(t, viewMaxRetriesInput, model.viewState)
		assert.Equal(t, "MULTIPLE_max_retries", model.pendingJobId)
		assert.Equal(t, "", model.maxRetriesInput.Value())
		m.selectedJobs = make(map[string]bool) // Reset
	})
}

func TestDashboardModel_UpdateMaxRetriesInput(t *testing.T) {
	m := NewDashboardModel("http://localhost:2112")
	m.viewState = viewMaxRetriesInput
	m.pendingJobId = "TEST-123"
	m.maxRetriesInput.Focus()
	m.maxRetriesInput.SetValue("")

	t.Run("Input Keystrokes", func(t *testing.T) {
		newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("5")})
		model := newM.(DashboardModel)
		assert.Equal(t, "5", model.maxRetriesInput.Value())
		m = model
	})

	t.Run("Cancel Input", func(t *testing.T) {
		newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
		model := newM.(DashboardModel)
		assert.Equal(t, viewMain, model.viewState)
		assert.Equal(t, "", model.pendingJobId)
	})

	t.Run("Submit Input - Empty", func(t *testing.T) {
		m.maxRetriesInput.SetValue("")
		newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		model := newM.(DashboardModel)
		assert.Error(t, model.err)
		assert.Contains(t, model.err.Error(), "cannot be empty")
	})

	t.Run("Submit Input - Invalid", func(t *testing.T) {
		m.maxRetriesInput.SetValue("abc")
		newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		model := newM.(DashboardModel)
		assert.Error(t, model.err)
		assert.Contains(t, model.err.Error(), "Invalid max retries value")
	})

	t.Run("Submit Input - Valid", func(t *testing.T) {
		m.maxRetriesInput.SetValue("5")
		newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		model := newM.(DashboardModel)
		assert.Equal(t, viewMain, model.viewState)
		assert.Equal(t, "", model.pendingJobId)
		assert.NotNil(t, cmd)

		msg := cmd()
		actionMsg, ok := msg.(actionMsg)
		assert.True(t, ok, "Expected actionMsg, got %T", msg)
		// It's going to hit the actual network, so the err will likely be 'connection refused',
		// but that means the command was correctly triggered. We just want to ensure it returns an actionMsg.
		if actionMsg.Err != nil {
			assert.Contains(t, actionMsg.Err.Error(), "connection refused")
		}
	})
}
