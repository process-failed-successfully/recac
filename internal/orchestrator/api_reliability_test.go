package orchestrator

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIReliabilityStats(t *testing.T) {
	orch := New(nil, nil, 1*time.Minute)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ctx := context.Background()

	// Seed completed jobs
	jobs := []JobInfo{
		{
			ID: "job-1",
			Status: "Completed",
			Summary: "Task A",
			RetryCount: 0,
		},
		{
			ID: "job-2",
			Status: "Completed",
			Summary: "Task A",
			RetryCount: 2,
		},
		{
			ID: "job-3",
			Status: "Failed",
			Summary: "Task B",
			RetryCount: 3,
		},
		{
			ID: "job-4",
			Status: "Completed",
			Summary: "Task B",
			RetryCount: 1,
		},
		{
			ID: "job-5",
			Status: "Canceled", // Should be ignored
			Summary: "Task C",
			RetryCount: 0,
		},
		{
			ID: "job-6",
			Status: "Completed",
			Summary: "Task D",
			RetryCount: 0,
		},
	}

	for _, j := range jobs {
		orch.addToHistory(j, nil)
	}

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, logger, ctx)

	server := httptest.NewServer(mux)
	defer server.Close()

	resp, err := http.Get(server.URL + "/jobs/analyze/reliability?limit=5")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var stats ReliabilityStats
	err = json.NewDecoder(resp.Body).Decode(&stats)
	require.NoError(t, err)

	// Total jobs excluding canceled/skipped
	assert.Equal(t, 5, stats.TotalJobs)
	assert.Equal(t, 2, stats.SuccessfulJobs) // job-1, job-6
	assert.Equal(t, 2, stats.FlakyJobs)      // job-2, job-4
	assert.Equal(t, 1, stats.FailedJobs)     // job-3
	assert.Equal(t, 6, stats.TotalRetries)   // 2 (job-2) + 3 (job-3) + 1 (job-4)

	assert.InDelta(t, 80.0, stats.SuccessRate, 0.01)  // 4 / 5
	assert.InDelta(t, 40.0, stats.FlakinessRate, 0.01) // 2 / 5
	assert.InDelta(t, 20.0, stats.FailureRate, 0.01)   // 1 / 5

	// Check top flaky jobs
	assert.Len(t, stats.TopFlakyJobs, 2)
	// Task A: 1 flaky occurrence, 2 total retries
	// Task B: 1 flaky occurrence, 1 total retry
	assert.Equal(t, "Task A", stats.TopFlakyJobs[0].Summary)
	assert.Equal(t, 1, stats.TopFlakyJobs[0].Occurrences)
	assert.Equal(t, 2, stats.TopFlakyJobs[0].TotalRetries)
	assert.Equal(t, 2.0, stats.TopFlakyJobs[0].AvgRetries)

	assert.Equal(t, "Task B", stats.TopFlakyJobs[1].Summary)
	assert.Equal(t, 1, stats.TopFlakyJobs[1].Occurrences)
	assert.Equal(t, 1, stats.TopFlakyJobs[1].TotalRetries)
	assert.Equal(t, 1.0, stats.TopFlakyJobs[1].AvgRetries)

	// Check top failing jobs
	assert.Len(t, stats.TopFailingJobs, 1)
	assert.Equal(t, "Task B", stats.TopFailingJobs[0].Summary)
	assert.Equal(t, 1, stats.TopFailingJobs[0].Occurrences)
}
