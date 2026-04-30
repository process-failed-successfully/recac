package tui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

type ContextLine struct {
	LineNumber int    `json:"line_number"`
	Text       string `json:"text"`
}

type LogMatch struct {
	LineNumber    int           `json:"line_number"`
	Text          string        `json:"text"`
	ContextBefore []ContextLine `json:"context_before,omitempty"`
	ContextAfter  []ContextLine `json:"context_after,omitempty"`
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
		contextLines := r.URL.Query().Get("context")
		if q == "error" {
			if contextLines == "5" {
				w.Header().Set("X-Test-Context", "5")
			}
			results := []JobLogResult{
				{
					JobID:   "JOB-1",
					Summary: "Test Job",
					Status:  "Failed",
					Matches: []LogMatch{
						{
							LineNumber: 42,
							Text:       "fatal error: panic",
							ContextBefore: []ContextLine{
								{LineNumber: 41, Text: "doing some work"},
							},
							ContextAfter: []ContextLine{
								{LineNumber: 43, Text: "stack trace follows"},
							},
						},
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

	// Press enter to move to context lines
	mModel, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mModel.(DashboardModel)
	assert.Equal(t, viewSearchLogsContextInput, m.viewState)
	assert.NotNil(t, cmd) // Blink command

	// Type context "5"
	mModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("5")})
	m = mModel.(DashboardModel)
	assert.Equal(t, "5", m.searchContextInput.Value())

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
	assert.Contains(t, searchMsg.Output, "doing some work")
	assert.Contains(t, searchMsg.Output, "stack trace follows")
	assert.Contains(t, searchMsg.Output, "---")

	// Pass result msg back to model
	mModel, _ = m.Update(searchMsg)
	m = mModel.(DashboardModel)
	assert.Equal(t, viewSearchLogsResult, m.viewState)
	assert.Contains(t, m.viewport.View(), "fatal error: panic")
	assert.Contains(t, m.viewport.View(), "doing some work")
	assert.Contains(t, m.viewport.View(), "stack trace follows")
	assert.Contains(t, m.viewport.View(), "---")
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

	// Press enter (transitions to context view)
	mModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mModel.(DashboardModel)
	assert.NotNil(t, cmd)

	// Press enter again to search
	mModel, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
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

	// Cancel from Context Input
	m.viewState = viewSearchLogsContextInput
	m.searchInput.SetValue("error")
	m.searchContextInput.SetValue("5")

	mModel, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = mModel.(DashboardModel)
	assert.Equal(t, viewMain, m.viewState)
	assert.Equal(t, "", m.searchInput.Value())
	assert.Equal(t, "", m.searchContextInput.Value())
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
