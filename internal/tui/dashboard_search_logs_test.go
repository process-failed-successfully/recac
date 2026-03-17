package tui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

type LogMatch struct {
	LineNumber int    `json:"line_number"`
	Text       string `json:"text"`
}

type JobLogResult struct {
	JobID   string     `json:"job_id"`
	Summary string     `json:"summary"`
	Status  string     `json:"status"`
	Matches []LogMatch `json:"matches"`
}

func TestDashboardSearchLogsInteractive(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/search/logs", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		if q == "error" {
			results := []JobLogResult{
				{
					JobID:   "JOB-1",
					Summary: "Test Job",
					Status:  "Failed",
					Matches: []LogMatch{
						{LineNumber: 42, Text: "fatal error: panic"},
					},
				},
			}
			json.NewEncoder(w).Encode(results)
			return
		} else if q == "nothing" {
			w.Write([]byte(`[]`))
			return
		}
		w.WriteHeader(http.StatusBadRequest)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	m := NewDashboardModel(server.URL)

	// Simulate pressing 'S' to open search logs input
	mModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("S")})
	m = mModel.(DashboardModel)
	assert.Equal(t, viewSearchLogsInput, m.viewState)

	// Type query "error"
	mModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("error")})
	m = mModel.(DashboardModel)
	assert.Equal(t, "error", m.searchInput.Value())

	// Press enter to search
	mModel, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mModel.(DashboardModel)
	assert.NotNil(t, cmd)

	// Execute cmd
	msg := cmd()

	// Expect searchLogsResultMsg
	searchMsg, ok := msg.(searchLogsResultMsg)
	assert.True(t, ok)
	assert.NoError(t, searchMsg.Err)
	assert.Contains(t, searchMsg.Output, "fatal error: panic")
	assert.Contains(t, searchMsg.Output, "JOB-1")

	// Pass result msg back to model
	mModel, _ = m.Update(searchMsg)
	m = mModel.(DashboardModel)
	assert.Equal(t, viewSearchLogsResult, m.viewState)
	assert.Contains(t, m.viewport.View(), "fatal error: panic")
}

func TestDashboardSearchLogsNoResults(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/search/logs", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[]`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	m := NewDashboardModel(server.URL)
	m.viewState = viewSearchLogsInput
	m.searchInput.SetValue("nothing")

	// Press enter
	mModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mModel.(DashboardModel)
	assert.NotNil(t, cmd)

	msg := cmd()
	searchMsg, ok := msg.(searchLogsResultMsg)
	assert.True(t, ok)
	assert.NoError(t, searchMsg.Err)
	assert.Contains(t, searchMsg.Output, "No matching logs found")

	mModel, _ = m.Update(searchMsg)
	m = mModel.(DashboardModel)
	assert.Equal(t, viewSearchLogsResult, m.viewState)
	assert.Contains(t, m.viewport.View(), "No matching logs found")
}

func TestDashboardSearchLogsCancel(t *testing.T) {
	m := NewDashboardModel("http://dummy")
	m.viewState = viewSearchLogsInput
	m.searchInput.SetValue("error")

	// Press Esc
	mModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = mModel.(DashboardModel)
	assert.Equal(t, viewMain, m.viewState)
	assert.Equal(t, "", m.searchInput.Value()) // Input cleared
	assert.Nil(t, cmd)
}

func TestDashboardSearchLogsEmptyInput(t *testing.T) {
	m := NewDashboardModel("http://dummy")
	m.viewState = viewSearchLogsInput
	m.searchInput.SetValue("")

	// Press enter on empty input
	mModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mModel.(DashboardModel)
	assert.Equal(t, viewSearchLogsInput, m.viewState) // Stays on input
	assert.Nil(t, cmd)                                // Does not dispatch search
}
