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

func TestAnalyzeDurationsAPI(t *testing.T) {
	poller := NewFilePoller("dummy.json")
	spawner := NewProcessSpawner(slog.Default(), poller, "provider", "model", nil, 3, 5, 10)
	orch := New(poller, spawner, 1*time.Minute)
	logger := slog.Default()

	ctx := context.Background()

	// 1. Job 1: 10 seconds duration
	start1 := time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC)
	end1 := start1.Add(10 * time.Second)
	orch.completedJobs = append(orch.completedJobs, JobInfo{
		ID:        "JOB-1",
		Status:    "Completed",
		StartTime: start1,
		EndTime:   end1,
		WorkItem: WorkItem{
			Tags: []string{"tag-a"},
		},
	})

	// 2. Job 2: 20 seconds duration
	start2 := time.Date(2023, 1, 1, 10, 5, 0, 0, time.UTC)
	end2 := start2.Add(20 * time.Second)
	orch.completedJobs = append(orch.completedJobs, JobInfo{
		ID:        "JOB-2",
		Status:    "Completed",
		StartTime: start2,
		EndTime:   end2,
		WorkItem: WorkItem{
			Tags: []string{"tag-a", "tag-b"},
		},
	})

	// Setup API
	mux := http.NewServeMux()
	RegisterAPI(mux, orch, logger, ctx)
	server := httptest.NewServer(mux)
	defer server.Close()

	// Perform request
	resp, err := http.Get(server.URL + "/jobs/analyze/durations")
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var stats struct {
		TotalJobs      int       `json:"total_jobs"`
		TotalDuration  float64   `json:"total_duration_ms"`
		MeanDuration   float64   `json:"mean_duration_ms"`
		MedianDuration float64   `json:"median_duration_ms"`
		MinDuration    float64   `json:"min_duration_ms"`
		MaxDuration    float64   `json:"max_duration_ms"`
		TagStats       []struct {
			Tag          string  `json:"tag"`
			Count        int     `json:"count"`
			MeanDuration float64 `json:"mean_duration_ms"`
		} `json:"tag_stats"`
		TopSlowest []JobInfo `json:"top_slowest"`
	}

	err = json.NewDecoder(resp.Body).Decode(&stats)
	assert.NoError(t, err)

	assert.Equal(t, 2, stats.TotalJobs)
	assert.Equal(t, 30000.0, stats.TotalDuration) // 30s
	assert.Equal(t, 15000.0, stats.MeanDuration)  // 15s
	assert.Equal(t, 15000.0, stats.MedianDuration) // 15s
	assert.Equal(t, 10000.0, stats.MinDuration)    // 10s
	assert.Equal(t, 20000.0, stats.MaxDuration)    // 20s

	// Tag stats check
	// tag-a should have 2 count, mean (10+20)/2 = 15s
	// tag-b should have 1 count, mean 20s
	assert.Len(t, stats.TagStats, 2)
	assert.Equal(t, "tag-b", stats.TagStats[0].Tag) // sorted by mean duration desc
	assert.Equal(t, 20000.0, stats.TagStats[0].MeanDuration)

	assert.Equal(t, "tag-a", stats.TagStats[1].Tag)
	assert.Equal(t, 15000.0, stats.TagStats[1].MeanDuration)

	// Top slowest check (default limit 10, should return both, JOB-2 first)
	assert.Len(t, stats.TopSlowest, 2)
	assert.Equal(t, "JOB-2", stats.TopSlowest[0].ID)
	assert.Equal(t, "JOB-1", stats.TopSlowest[1].ID)

	// Test limit
	respLimit, errLimit := http.Get(server.URL + "/jobs/analyze/durations?limit=1")
	assert.NoError(t, errLimit)
	defer respLimit.Body.Close()
	assert.Equal(t, http.StatusOK, respLimit.StatusCode)

	var statsLimit struct {
		TopSlowest []JobInfo `json:"top_slowest"`
	}
	err = json.NewDecoder(respLimit.Body).Decode(&statsLimit)
	assert.NoError(t, err)
	assert.Len(t, statsLimit.TopSlowest, 1)
	assert.Equal(t, "JOB-2", statsLimit.TopSlowest[0].ID)
}
