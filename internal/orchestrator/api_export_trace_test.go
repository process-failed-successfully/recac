package orchestrator

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIExportTrace(t *testing.T) {
	poller := &MockPoller{}
	spawner := &MockSpawner{}
	orch := New(poller, spawner, time.Second)

	now := time.Now()

	orch.activeJobs["job-1"] = JobInfo{
		ID:        "job-1",
		Status:    "Running",
		StartTime: now.Add(-10 * time.Minute),
	}
	orch.completedJobs = []JobInfo{
		{
			ID:        "job-2",
			Status:    "Completed",
			StartTime: now.Add(-20 * time.Minute),
			EndTime:   now.Add(-15 * time.Minute),
		},
		{
			ID:        "job-3",
			Status:    "Failed",
			StartTime: now.Add(-5 * time.Minute),
			EndTime:   now.Add(-2 * time.Minute),
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mux := http.NewServeMux()
	RegisterAPI(mux, orch, logger, context.Background())
	server := httptest.NewServer(mux)
	defer server.Close()

	tests := []struct {
		name          string
		stateFilter   string
		expectedCount int
	}{
		{"All (default)", "", 3},
		{"All (explicit)", "all", 3},
		{"Active", "active", 1},
		{"Completed", "completed", 1},
		{"Failed", "failed", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := server.URL + "/jobs/export/trace"
			if tt.stateFilter != "" {
				url += "?state=" + tt.stateFilter
			}

			resp, err := http.Get(url)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
			assert.Contains(t, resp.Header.Get("Content-Disposition"), "attachment; filename=trace.json")

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			var events []TraceEvent
			err = json.Unmarshal(body, &events)
			require.NoError(t, err)

			assert.Len(t, events, tt.expectedCount)

			if tt.stateFilter == "active" {
				assert.Equal(t, "job-1", events[0].Name)
			} else if tt.stateFilter == "completed" {
				assert.Equal(t, "job-2", events[0].Name)
			} else if tt.stateFilter == "failed" {
				assert.Equal(t, "job-3", events[0].Name)
			}
		})
	}
}
