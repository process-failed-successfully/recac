package tui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"recac/internal/orchestrator"
)

func TestDashboard_ExplainAction(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/status") {
			json.NewEncoder(w).Encode(orchestrator.Status{})
			return
		}
		if strings.HasSuffix(r.URL.Path, "/jobs") {
			json.NewEncoder(w).Encode([]orchestrator.JobInfo{
				{
					ID:      "JOB-EXPLAIN-1",
					Summary: "Failed Job",
					Status:  "Failed",
				},
			})
			return
		}
		if strings.HasSuffix(r.URL.Path, "/jobs/JOB-EXPLAIN-1/explain") {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"explanation": "This is a simulated explanation."})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	m := NewDashboardModel(ts.URL)

	// Fetch status and jobs to populate the table
	cmd := fetchStatus(ts.URL, false)
	msg := cmd()
	newM, _ := m.Update(msg)
	m = newM.(DashboardModel)

	// Verify job is in the table
	require.Len(t, m.jobs, 1)

	// Move selection down to the job (index 0) - bubbletea table starts selected at 0 if rows > 0
	// Table is populated via updateTableContent(), which is called in Update on statusMsg

	// Send '?' key message to trigger fetchExplanation cmd
	newM, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = newM.(DashboardModel)
	require.NotNil(t, cmd)

	// Execute fetchExplanation cmd
	explainMsgResult := cmd()
	require.IsType(t, explainMsg{}, explainMsgResult)
	explainData := explainMsgResult.(explainMsg)
	require.NoError(t, explainData.Err)
	assert.Equal(t, "This is a simulated explanation.", explainData.Explanation)

	// Update with explainMsg
	newM, _ = m.Update(explainData)
	m = newM.(DashboardModel)

	// Verify view state and content
	assert.Equal(t, viewExplain, m.viewState)
	assert.Equal(t, "This is a simulated explanation.", m.explain)
	assert.Contains(t, m.viewport.View(), "This is a simulated explanation.")
}
