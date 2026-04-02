package orchestrator

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"context"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"log/slog"
)

func TestHandleAnalyzeCosts(t *testing.T) {
	orch := New(&MockPoller{}, &MockSpawner{}, time.Minute)

	orch.mu.Lock()
	orch.completedJobs = []JobInfo{
		{
			ID:      "job1",
			Summary: "Job 1",
			WorkItem: WorkItem{
				AgentModel: "gpt-4",
				Tags:       []string{"backend", "db"},
			},
			Metrics: map[string]float64{
				"cost_usd":          1.5,
				"tokens_prompt":     1000,
				"tokens_completion": 500,
			},
		},
		{
			ID:      "job2",
			Summary: "Job 2",
			WorkItem: WorkItem{
				AgentModel: "gpt-3.5",
				Tags:       []string{"frontend"},
			},
			Metrics: map[string]float64{
				"cost_usd":          0.5,
				"tokens_prompt":     2000,
				"tokens_completion": 200,
			},
		},
		{
			ID:      "job3",
			Summary: "Job 3",
			WorkItem: WorkItem{
				AgentModel: "gpt-4",
				Tags:       []string{"backend"},
			},
			Metrics: map[string]float64{
				"cost_usd":          2.0,
				"tokens_prompt":     1500,
				"tokens_completion": 600,
			},
		},
		{
			ID:      "job-no-cost",
			Summary: "Job No Cost",
			WorkItem: WorkItem{
				AgentModel: "gpt-3.5",
				Tags:       []string{"frontend"},
			},
			Metrics: map[string]float64{
				"other_metric": 123,
			},
		},
	}
	orch.mu.Unlock()

	mux := http.NewServeMux()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	RegisterAPI(mux, orch, logger, context.Background())

	server := httptest.NewServer(mux)
	defer server.Close()

	resp, err := http.Get(server.URL + "/jobs/analyze/costs")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result CostStatsResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	// Verify Total Stats
	assert.Equal(t, 3, result.TotalStats.TotalJobs)
	assert.Equal(t, 4.0, result.TotalStats.TotalCost)
	assert.Equal(t, 4500.0, result.TotalStats.TotalTokensPrompt)
	assert.Equal(t, 1300.0, result.TotalStats.TotalTokensCompletion)

	// Verify Model Stats
	assert.Len(t, result.ModelStats, 2)
	assert.Equal(t, "gpt-4", result.ModelStats[0].Model)
	assert.Equal(t, 3.5, result.ModelStats[0].Cost)
	assert.Equal(t, 2, result.ModelStats[0].JobsCount)

	assert.Equal(t, "gpt-3.5", result.ModelStats[1].Model)
	assert.Equal(t, 0.5, result.ModelStats[1].Cost)
	assert.Equal(t, 1, result.ModelStats[1].JobsCount)

	// Verify Tag Stats
	assert.Len(t, result.TagStats, 3)
	assert.Equal(t, "backend", result.TagStats[0].Tag)
	assert.Equal(t, 3.5, result.TagStats[0].Cost)
	assert.Equal(t, 2, result.TagStats[0].JobsCount)

	// Verify Top Expensive Jobs
	assert.Len(t, result.TopExpensiveJobs, 3)
	assert.Equal(t, "job3", result.TopExpensiveJobs[0].ID)
	assert.Equal(t, "job1", result.TopExpensiveJobs[1].ID)
	assert.Equal(t, "job2", result.TopExpensiveJobs[2].ID)
}
