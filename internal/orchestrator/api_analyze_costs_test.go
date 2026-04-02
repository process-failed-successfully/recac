package orchestrator

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAPIAnalyzeCosts(t *testing.T) {
	poller := NewFilePoller("dummy.json")
	spawner := NewProcessSpawner(slog.Default(), poller, "provider", "model", nil, 3, 5, 10)
	orch := New(poller, spawner, 1*time.Minute)
	logger := slog.Default()

	ctx := context.Background()

	// 1. Job 1
	orch.completedJobs = append(orch.completedJobs, JobInfo{
		ID:        "JOB-1",
		Summary:   "Summary 1",
		Status:    "Completed",
		WorkItem: WorkItem{
			Tags:       []string{"tag-a"},
			AgentModel: "model-x",
		},
		Metrics: map[string]float64{
			"cost":   1.5,
			"tokens": 1500,
		},
	})

	// 2. Job 2
	orch.completedJobs = append(orch.completedJobs, JobInfo{
		ID:        "JOB-2",
		Summary:   "Summary 2",
		Status:    "Completed",
		WorkItem: WorkItem{
			Tags:       []string{"tag-a", "tag-b"},
			AgentModel: "model-y",
		},
		Metrics: map[string]float64{
			"cost":   3.5,
			"tokens": 3500,
		},
	})

	// 3. Job 3 (Failed, but has cost)
	orch.completedJobs = append(orch.completedJobs, JobInfo{
		ID:        "JOB-3",
		Summary:   "Summary 3",
		Status:    "Failed",
		WorkItem: WorkItem{
			Tags:       []string{"tag-c"},
			AgentModel: "model-x",
		},
		Metrics: map[string]float64{
			"cost":   0.5,
			"tokens": 500,
		},
	})

	// Setup API
	mux := http.NewServeMux()
	RegisterAPI(mux, orch, logger, ctx)
	server := httptest.NewServer(mux)
	defer server.Close()

	// Perform request
	resp, err := http.Get(server.URL + "/jobs/analyze/costs")
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var stats struct {
		TotalJobs      int     `json:"total_jobs"`
		TotalCost      float64 `json:"total_cost"`
		TotalTokens    float64 `json:"total_tokens"`
		CostByTag      []struct {
			Tag   string  `json:"tag"`
			Cost  float64 `json:"cost"`
			Count int     `json:"count"`
		} `json:"cost_by_tag"`
		CostByModel []struct {
			Model string  `json:"model"`
			Cost  float64 `json:"cost"`
			Count int     `json:"count"`
		} `json:"cost_by_model"`
		TopExpensiveJobs []JobInfo `json:"top_expensive_jobs"`
	}

	err = json.NewDecoder(resp.Body).Decode(&stats)
	assert.NoError(t, err)

	assert.Equal(t, 3, stats.TotalJobs)
	assert.Equal(t, 5.5, stats.TotalCost)
	assert.Equal(t, 5500.0, stats.TotalTokens)

	// Cost by Tag check (sorted by cost desc)
	// tag-a: 1.5 + 3.5 = 5.0
	// tag-b: 3.5
	// tag-c: 0.5
	assert.Len(t, stats.CostByTag, 3)
	assert.Equal(t, "tag-a", stats.CostByTag[0].Tag)
	assert.Equal(t, 5.0, stats.CostByTag[0].Cost)
	assert.Equal(t, "tag-b", stats.CostByTag[1].Tag)
	assert.Equal(t, 3.5, stats.CostByTag[1].Cost)
	assert.Equal(t, "tag-c", stats.CostByTag[2].Tag)
	assert.Equal(t, 0.5, stats.CostByTag[2].Cost)

	// Cost by Model check (sorted by cost desc)
	// model-y: 3.5
	// model-x: 1.5 + 0.5 = 2.0
	assert.Len(t, stats.CostByModel, 2)
	assert.Equal(t, "model-y", stats.CostByModel[0].Model)
	assert.Equal(t, 3.5, stats.CostByModel[0].Cost)
	assert.Equal(t, "model-x", stats.CostByModel[1].Model)
	assert.Equal(t, 2.0, stats.CostByModel[1].Cost)

	// Top Expensive Jobs
	assert.Len(t, stats.TopExpensiveJobs, 3)
	assert.Equal(t, "JOB-2", stats.TopExpensiveJobs[0].ID)
	assert.Equal(t, "JOB-1", stats.TopExpensiveJobs[1].ID)
	assert.Equal(t, "JOB-3", stats.TopExpensiveJobs[2].ID)

	// Test limit
	respLimit, errLimit := http.Get(server.URL + "/jobs/analyze/costs?limit=1")
	assert.NoError(t, errLimit)
	defer respLimit.Body.Close()
	assert.Equal(t, http.StatusOK, respLimit.StatusCode)

	var statsLimit struct {
		TopExpensiveJobs []JobInfo `json:"top_expensive_jobs"`
	}
	err = json.NewDecoder(respLimit.Body).Decode(&statsLimit)
	assert.NoError(t, err)
	assert.Len(t, statsLimit.TopExpensiveJobs, 1)
	assert.Equal(t, "JOB-2", statsLimit.TopExpensiveJobs[0].ID)
}
