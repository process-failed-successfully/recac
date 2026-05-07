package tui

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"github.com/stretchr/testify/assert"
	"recac/internal/orchestrator"
)

func TestDashboard_AnalyzeCosts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs/analyze/costs" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"total_stats": {"total_jobs": 5, "total_cost": 12.34}}`))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	m := NewDashboardModel(server.URL)
	cmd := fetchAnalyzeCostsCmd(server.URL)
	msg := cmd()
	newM, _ := m.Update(msg)
	m = newM.(DashboardModel)
	assert.Equal(t, viewAnalyzeCosts, m.viewState)

	assert.Contains(t, m.viewport.View(), "12.34")
}

func TestFetchAnalyzeCosts_Err(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	cmd := fetchAnalyzeCostsCmd(ts.URL)
	msg := cmd()
	_, ok := msg.(analyzeCostsMsg)
	assert.True(t, ok)
}

func TestFetchAnalyzeCosts_BadJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`bad json`))
	}))
	defer ts.Close()

	cmd := fetchAnalyzeCostsCmd(ts.URL)
	msg := cmd()
	_, ok := msg.(analyzeCostsMsg)
	assert.True(t, ok)
}

func TestRenderAnalyzeCosts_Empty(t *testing.T) {
	view := renderAnalyzeCosts(CostStatsResponse{})
	assert.Contains(t, view, "No valid completed jobs")
}

func TestRenderAnalyzeCosts_Full(t *testing.T) {
    stats := CostStatsResponse{
        TotalStats: CostStats{
            TotalJobs: 5,
            TotalCost: 99.99,
        },
        TagStats: []CostByTag{
            {Tag: "t1", Cost: 10.0},
        },
        ModelStats: []CostByModel{
            {Model: "gpt-4", Cost: 80.0},
        },
        TopExpensiveJobs: []orchestrator.JobInfo{
            {ID: "JOB-1", Metrics: map[string]float64{"cost": 50.0}},
        },
    }
    view := renderAnalyzeCosts(stats)
    assert.Contains(t, view, "99.99")
    assert.Contains(t, view, "t1")
    assert.Contains(t, view, "gpt-4")
    assert.Contains(t, view, "JOB-1")
}
