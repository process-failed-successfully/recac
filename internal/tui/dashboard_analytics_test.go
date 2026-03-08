package tui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"recac/internal/orchestrator"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDashboardModel_Analytics(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/analytics" {
			w.WriteHeader(http.StatusOK)
			analytics := orchestrator.Analytics{
				TotalJobs:      10,
				SuccessfulJobs: 8,
				FailedJobs:     2,
				CanceledJobs:   0,
				SuccessRate:    80.0,
			}
			json.NewEncoder(w).Encode(analytics)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	m := NewDashboardModel(server.URL)

	// Test fetch cmd
	cmd := fetchAnalytics(server.URL)
	msg := cmd()
	am, ok := msg.(analyticsMsg)
	assert.True(t, ok)
	assert.NoError(t, am.Err)
	assert.Equal(t, 10, am.Analytics.TotalJobs)

	// Update model with analyticsMsg
	newModel, _ := m.Update(am)
	dm, ok := newModel.(DashboardModel)
	assert.True(t, ok)

	// Assert viewState is updated
	assert.Equal(t, viewAnalytics, dm.viewState)

	// Test render contains stats
	view := dm.View()
	assert.Contains(t, view, "Orchestrator Analytics")
	assert.Contains(t, view, "Total Jobs")
	assert.Contains(t, view, "10")
	assert.Contains(t, view, "80.00%")
}
