package orchestrator

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHandleAnalyzeTags(t *testing.T) {
	orch := New(nil, nil, time.Minute)
	logger := slog.Default()

	now := time.Now()
	orch.completedJobs = []JobInfo{
		{
			ID: "job-1",
			WorkItem: WorkItem{Tags: []string{"tag1", "tag2"}},
			Status: "Completed",
			StartTime: now.Add(-10 * time.Minute),
			EndTime: now,
			Metrics: map[string]float64{"cost_usd": 0.5, "tokens_total": 100},
		},
		{
			ID: "job-2",
			WorkItem: WorkItem{Tags: []string{"tag1", "tag3"}},
			Status: "Failed",
			StartTime: now.Add(-5 * time.Minute),
			EndTime: now,
			Metrics: map[string]float64{"cost_usd": 0.1, "tokens_total": 20},
		},
	}

	handler := handleAnalyzeTags(orch, logger)

	req := httptest.NewRequest("GET", "/jobs/analyze/tags?limit=2", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status %v, got %v", http.StatusOK, w.Code)
	}

	var resp TagStatsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(resp.Tags) != 2 {
		t.Fatalf("Expected 2 tags (due to limit), got %v", len(resp.Tags))
	}

	// tag1 should be first as it has 2 jobs
	if resp.Tags[0].Tag != "tag1" {
		t.Errorf("Expected first tag to be tag1, got %v", resp.Tags[0].Tag)
	}
	if resp.Tags[0].TotalJobs != 2 {
		t.Errorf("Expected tag1 to have 2 total jobs, got %v", resp.Tags[0].TotalJobs)
	}
	if resp.Tags[0].TotalCost != 0.6 {
		t.Errorf("Expected tag1 to have 0.6 total cost, got %v", resp.Tags[0].TotalCost)
	}
	if resp.Tags[0].TotalTokens != 120 {
		t.Errorf("Expected tag1 to have 120 total tokens, got %v", resp.Tags[0].TotalTokens)
	}
}
