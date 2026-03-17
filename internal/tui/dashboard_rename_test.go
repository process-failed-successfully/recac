package tui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"recac/internal/orchestrator"
	"testing"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestDashboardModel_RenameJob(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/JOB-1/rename", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		var req struct {
			NewID string `json:"new_id"`
		}
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, "NEW-JOB-1", req.NewID)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`"Renamed job JOB-1 to NEW-JOB-1"`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	model := NewDashboardModel(server.URL)
	model.jobs = []orchestrator.JobInfo{
		{ID: "JOB-1", Summary: "Test Job"},
	}
	model.table.SetRows([]table.Row{{"JOB-1", "Test Job"}})
	model.table.SetCursor(0)

	// Trigger rename view
	updatedModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("N")})
	assert.NotNil(t, cmd) // textinput.Blink
	m := updatedModel.(DashboardModel)
	assert.Equal(t, viewRenameInput, m.viewState)
	assert.Equal(t, "JOB-1", m.pendingJobId)
	assert.Equal(t, "JOB-1", m.renameInput.Value())

	// Simulate input
	m.renameInput.SetValue("NEW-JOB-1")

	// Submit input
	updatedModel, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updatedModel.(DashboardModel)
	assert.Equal(t, viewMain, m.viewState)
	assert.NotNil(t, cmd)

	// Execute command
	msg := cmd()
	actionMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.NoError(t, actionMsg.Err)
	assert.Contains(t, actionMsg.Message, "Renamed job JOB-1 to NEW-JOB-1")
}
