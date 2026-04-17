package tui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDashboardModel_AnalyzeAgents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs/analyze/agents" {
			w.WriteHeader(http.StatusOK)
			stats := AgentStatsResponse{
				Agents: []AgentPerformance{
					{
						AgentProvider:   "openai",
						AgentModel:      "gpt-4",
						TotalJobs:       10,
						SuccessRate:     90.0,
						AverageDuration: time.Minute,
						AverageCost:     1.20,
						TotalCost:       12.00,
						TotalTokens:     100000,
					},
					{
						AgentProvider:   "openai",
						AgentModel:      "gpt-3.5",
						TotalJobs:       20,
						SuccessRate:     95.0,
						AverageDuration: 30 * time.Second,
						AverageCost:     0.025,
						TotalCost:       0.50,
						TotalTokens:     50000,
					},
				},
			}
			json.NewEncoder(w).Encode(stats)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	m := NewDashboardModel(server.URL)

	cmd := fetchAnalyzeAgentsCmd(server.URL)
	msg := cmd()
	aam, ok := msg.(analyzeAgentsMsg)
	assert.True(t, ok)
	assert.NoError(t, aam.Err)
	assert.Equal(t, 2, len(aam.Stats.Agents))
	assert.Equal(t, "gpt-4", aam.Stats.Agents[0].AgentModel)

	newModel, _ := m.Update(aam)
	dm, ok := newModel.(DashboardModel)
	assert.True(t, ok)

	assert.Equal(t, viewAnalyzeAgents, dm.viewState)

	view := dm.View()
	assert.Contains(t, view, "AI Agent Performance Analysis")
	assert.Contains(t, view, "gpt-4")
	assert.Contains(t, view, "openai")
	assert.Contains(t, view, "10")
	assert.Contains(t, view, "9000.0%")
	assert.Contains(t, view, "$1.2000")
	assert.Contains(t, view, "$12")
	assert.Contains(t, view, "gpt-3.5")
	assert.Contains(t, view, "20")
	assert.Contains(t, view, "9500.0%")
	assert.Contains(t, view, "$0.0250")
	assert.Contains(t, view, "$0.")
}

func TestDashboardModel_AnalyzeAgents_Empty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs/analyze/agents" {
			w.WriteHeader(http.StatusOK)
			stats := AgentStatsResponse{
				Agents: []AgentPerformance{},
			}
			json.NewEncoder(w).Encode(stats)
		}
	}))
	defer server.Close()

	m := NewDashboardModel(server.URL)

	cmd := fetchAnalyzeAgentsCmd(server.URL)
	msg := cmd()
	aam, ok := msg.(analyzeAgentsMsg)
	assert.True(t, ok)
	assert.NoError(t, aam.Err)

	newModel, _ := m.Update(aam)
	dm, ok := newModel.(DashboardModel)
	assert.True(t, ok)

	assert.Equal(t, viewAnalyzeAgents, dm.viewState)

	view := dm.View()
	assert.Contains(t, view, "No valid completed jobs with agent data found.")
}

func TestDashboardModel_AnalyzeAgents_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	cmd := fetchAnalyzeAgentsCmd(server.URL)
	msg := cmd()
	aam, ok := msg.(analyzeAgentsMsg)
	assert.True(t, ok)
	assert.Error(t, aam.Err)
	assert.Contains(t, aam.Err.Error(), "status 500")
}
