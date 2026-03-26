package tui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"recac/internal/orchestrator"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDashboardModel_AnalyzeFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs" && r.URL.Query().Get("status") == "Failed" {
			w.WriteHeader(http.StatusOK)
			jobs := []orchestrator.JobInfo{
				{
					ID:      "JOB-1",
					Summary: "Panic error in main",
					Status:  "Failed",
				},
				{
					ID:      "JOB-2",
					Summary: "Panic error in main",
					Status:  "Failed",
				},
				{
					ID:      "JOB-3",
					Summary: "Network timeout",
					Status:  "Failed",
				},
			}
			json.NewEncoder(w).Encode(jobs)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	m := NewDashboardModel(server.URL)

	// Test fetch cmd
	cmd := fetchAnalyzeFailuresCmd(server.URL)
	msg := cmd()
	afm, ok := msg.(analyzeFailuresMsg)
	assert.True(t, ok)
	assert.NoError(t, afm.Err)
	assert.Len(t, afm.FailedJobs, 3)

	// Update model with analyzeFailuresMsg
	newModel, _ := m.Update(afm)
	dm, ok := newModel.(DashboardModel)
	assert.True(t, ok)

	// Assert viewState is updated
	assert.Equal(t, viewAnalyzeFailures, dm.viewState)

	// Test render contains grouped stats
	view := dm.View()
	assert.Contains(t, view, "Failed Jobs Analysis (3 total)")
	assert.Contains(t, view, "Panic error in main")
	assert.Contains(t, view, "2") // Count for Panic error
	assert.Contains(t, view, "Network timeout")
	assert.Contains(t, view, "1") // Count for Network timeout
	assert.Contains(t, view, "JOB-1, JOB-2")
	assert.Contains(t, view, "JOB-3")
}

func TestDashboardModel_AnalyzeFailures_Empty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]orchestrator.JobInfo{})
		}
	}))
	defer server.Close()

	m := NewDashboardModel(server.URL)

	// Test fetch cmd
	cmd := fetchAnalyzeFailuresCmd(server.URL)
	msg := cmd()
	afm, ok := msg.(analyzeFailuresMsg)
	assert.True(t, ok)
	assert.NoError(t, afm.Err)
	assert.Len(t, afm.FailedJobs, 0)

	// Update model
	newModel, _ := m.Update(afm)
	dm, ok := newModel.(DashboardModel)
	assert.True(t, ok)

	// Assert view
	view := dm.View()
	assert.Contains(t, view, "No failed jobs found.")
}

func TestDashboardModel_AnalyzeFailures_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	cmd := fetchAnalyzeFailuresCmd(server.URL)
	msg := cmd()
	afm, ok := msg.(analyzeFailuresMsg)
	assert.True(t, ok)
	assert.Error(t, afm.Err)
	assert.Contains(t, afm.Err.Error(), "status 500")
}
