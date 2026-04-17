package tui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDashboardModel_AnalyzeCosts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs/analyze/costs" {
			w.WriteHeader(http.StatusOK)
			stats := CostStatsResponse{
				TotalStats: CostStats{
					TotalCost:             12.50,
					TotalTokensPrompt:     50000,
					TotalTokensCompletion: 50000,
					TotalJobs:             2,
				},
				TagStats: []CostByTag{
					{Tag: "backend", Cost: 10.00},
					{Tag: "frontend", Cost: 2.50},
				},
				ModelStats: []CostByModel{
					{Model: "gpt-4", Cost: 12.00},
					{Model: "gpt-3.5", Cost: 0.50},
				},
			}
			json.NewEncoder(w).Encode(stats)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	m := NewDashboardModel(server.URL)

	cmd := fetchAnalyzeCostsCmd(server.URL)
	msg := cmd()
	acm, ok := msg.(analyzeCostsMsg)
	assert.True(t, ok)
	assert.NoError(t, acm.Err)
	assert.Equal(t, 12.50, acm.Stats.TotalStats.TotalCost)

	newModel, _ := m.Update(acm)
	dm, ok := newModel.(DashboardModel)
	assert.True(t, ok)

	assert.Equal(t, viewAnalyzeCosts, dm.viewState)

	view := dm.View()
	assert.Contains(t, view, "AI Cost Analysis")
	assert.Contains(t, view, "Total Cost:")
	assert.Contains(t, view, "$12.50")
	assert.Contains(t, view, "Cost by Tag")
	assert.Contains(t, view, "backend")
	assert.Contains(t, view, "$10.00")
	assert.Contains(t, view, "Cost by Model")
}

func TestDashboardModel_AnalyzeCosts_Empty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs/analyze/costs" {
			w.WriteHeader(http.StatusOK)
			stats := CostStatsResponse{
				TotalStats: CostStats{
					TotalCost: 0,
					TotalJobs: 0,
				},
			}
			json.NewEncoder(w).Encode(stats)
		}
	}))
	defer server.Close()

	m := NewDashboardModel(server.URL)

	cmd := fetchAnalyzeCostsCmd(server.URL)
	msg := cmd()
	acm, ok := msg.(analyzeCostsMsg)
	assert.True(t, ok)
	assert.NoError(t, acm.Err)

	newModel, _ := m.Update(acm)
	dm, ok := newModel.(DashboardModel)
	assert.True(t, ok)

	assert.Equal(t, viewAnalyzeCosts, dm.viewState)

	view := dm.View()
	assert.Contains(t, view, "No valid completed jobs with cost data found")
}

func TestDashboardModel_AnalyzeCosts_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	cmd := fetchAnalyzeCostsCmd(server.URL)
	msg := cmd()
	acm, ok := msg.(analyzeCostsMsg)
	assert.True(t, ok)
	assert.Error(t, acm.Err)
	assert.Contains(t, acm.Err.Error(), "status 500")
}
