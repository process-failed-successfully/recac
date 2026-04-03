package orchestrator

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHandleAnalyzeAgents(t *testing.T) {
	poller := newMockPoller(nil)
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 100*time.Millisecond)

	now := time.Now()
	startTime1 := now.Add(-10 * time.Minute)
	endTime1 := now.Add(-5 * time.Minute)

	startTime2 := now.Add(-20 * time.Minute)
	endTime2 := now.Add(-10 * time.Minute)

	orch.completedJobs = []JobInfo{
		{
			ID: "JOB-1",
			WorkItem: WorkItem{
				AgentProvider: "openrouter",
				AgentModel:    "gpt-4",
			},
			Status:    "Completed",
			StartTime: startTime1,
			EndTime:   endTime1,
			Metrics: map[string]float64{
				"cost_usd":      0.50,
				"tokens_prompt": 100,
				"tokens_completion": 50,
			},
		},
		{
			ID: "JOB-2",
			WorkItem: WorkItem{
				AgentProvider: "openrouter",
				AgentModel:    "gpt-4",
			},
			Status:    "Failed",
			StartTime: startTime2,
			EndTime:   endTime2,
			Metrics: map[string]float64{
				"cost_usd":      0.10,
				"tokens_total": 120,
			},
		},
		{
			ID: "JOB-3",
			WorkItem: WorkItem{
				AgentProvider: "openai",
				AgentModel:    "gpt-3.5",
			},
			Status:    "Completed",
			StartTime: startTime1,
			EndTime:   endTime1,
			Metrics: map[string]float64{
				"cost_usd": 0.05,
			},
		},
	}

	handler := handleAnalyzeAgents(orch, silentLogger)
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err := http.Get(server.URL + "?limit=5")
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result AgentStatsResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	assert.NoError(t, err)

	assert.Len(t, result.Agents, 2)

	var gpt4 *AgentPerformance
	var gpt35 *AgentPerformance

	for i, agent := range result.Agents {
		if agent.AgentModel == "gpt-4" {
			gpt4 = &result.Agents[i]
		} else if agent.AgentModel == "gpt-3.5" {
			gpt35 = &result.Agents[i]
		}
	}

	assert.NotNil(t, gpt4)
	assert.Equal(t, "openrouter", gpt4.AgentProvider)
	assert.Equal(t, 2, gpt4.TotalJobs)
	assert.Equal(t, 1, gpt4.SuccessfulJobs)
	assert.Equal(t, 1, gpt4.FailedJobs)
	assert.Equal(t, 0.5, gpt4.SuccessRate)
	assert.Equal(t, 0.60, gpt4.TotalCost)
	assert.Equal(t, 0.30, gpt4.AverageCost)
	assert.Equal(t, 270.0, gpt4.TotalTokens)
	assert.Equal(t, 7*time.Minute+30*time.Second, gpt4.AverageDuration)

	assert.NotNil(t, gpt35)
	assert.Equal(t, "openai", gpt35.AgentProvider)
	assert.Equal(t, 1, gpt35.TotalJobs)
	assert.Equal(t, 1, gpt35.SuccessfulJobs)
	assert.Equal(t, 1.0, gpt35.SuccessRate)
	assert.Equal(t, 0.05, gpt35.TotalCost)
	assert.Equal(t, 5*time.Minute, gpt35.AverageDuration)
}
