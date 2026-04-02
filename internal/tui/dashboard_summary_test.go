package tui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestDashboardSummary(t *testing.T) {
	mux := http.NewServeMux()

	// Mock basic endpoints required by dashboard init
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	})
	mux.HandleFunc("/jobs", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	})

	// Mock the summary endpoint
	mux.HandleFunc("/jobs/summary", func(w http.ResponseWriter, r *http.Request) {
		summary := map[string]int{
			"Completed": 10,
			"Failed":    2,
			"Pending":   5,
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(summary)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	// Initialize dashboard
	m := NewDashboardModel(server.URL)
	m.table.SetWidth(100)
	m.table.SetHeight(20)

	// Send ctrl+u
	msg := tea.KeyMsg{Type: tea.KeyCtrlU}
	newModel, cmd := m.Update(msg)
	m = newModel.(DashboardModel)

	// Ensure a command was returned
	assert.NotNil(t, cmd)

	// Execute the command to fetch the summary
	fetchMsg := cmd()
	assert.IsType(t, summaryMsg{}, fetchMsg)

	smMsg := fetchMsg.(summaryMsg)
	assert.NoError(t, smMsg.Err)
	assert.Equal(t, 10, smMsg.Summary["Completed"])
	assert.Equal(t, 2, smMsg.Summary["Failed"])
	assert.Equal(t, 5, smMsg.Summary["Pending"])

	// Send the resulting message back into the model to trigger state change
	newModel, cmd = m.Update(smMsg)
	m = newModel.(DashboardModel)

	// Verify state transition to viewSummary
	assert.Equal(t, viewSummary, m.viewState)

	// Verify the view contains the expected content
	viewOutput := m.View()
	assert.Contains(t, viewOutput, "Job Summary (17 total)")
	assert.Contains(t, viewOutput, "Completed")
	assert.Contains(t, viewOutput, "Failed")
	assert.Contains(t, viewOutput, "Pending")
	assert.Contains(t, viewOutput, "10")
	assert.Contains(t, viewOutput, "2")
	assert.Contains(t, viewOutput, "5")
}
