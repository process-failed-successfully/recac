package orchestrator

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"log/slog"
)

func TestAPI_ExportTimeline(t *testing.T) {
	orch := New(&MockPoller{}, &MockSpawner{}, time.Minute)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Add some jobs to the orchestrator to test the endpoint
	now := time.Now()

	orch.activeJobs["job-1"] = JobInfo{
		ID:        "job-1",
		Status:    "Running",
		StartTime: now.Add(-5 * time.Minute),
	}

	orch.completedJobs = append(orch.completedJobs, JobInfo{
		ID:        "job-2",
		Status:    "Completed",
		StartTime: now.Add(-10 * time.Minute),
		EndTime:   now.Add(-2 * time.Minute),
	})

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, logger, context.Background())
	server := httptest.NewServer(mux)
	defer server.Close()

	t.Run("Default State (All)", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/jobs/export/timeline")
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read response body: %v", err)
		}

		timeline := string(body)
		if !strings.Contains(timeline, "gantt") {
			t.Errorf("Expected output to contain 'gantt'")
		}
		if !strings.Contains(timeline, "job-1") {
			t.Errorf("Expected output to contain job-1")
		}
		if !strings.Contains(timeline, "job-2") {
			t.Errorf("Expected output to contain job-2")
		}
	})

	t.Run("Active State", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/jobs/export/timeline?state=active")
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		timeline := string(body)

		if !strings.Contains(timeline, "job-1") {
			t.Errorf("Expected active jobs to contain job-1")
		}
		if strings.Contains(timeline, "job-2") {
			t.Errorf("Did not expect active jobs to contain job-2")
		}
	})

	t.Run("Completed State", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/jobs/export/timeline?state=completed")
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		timeline := string(body)

		if !strings.Contains(timeline, "job-2") {
			t.Errorf("Expected completed jobs to contain job-2")
		}
		if strings.Contains(timeline, "job-1") {
			t.Errorf("Did not expect completed jobs to contain job-1")
		}
	})
}
