package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"recac/internal/orchestrator"
	"recac/internal/tui"
	"strings"
	"testing"
	"time"
)

func executeJobCommand(args ...string) (string, error) {
	// Create a fresh command instance for each execution
	cmd := NewJobCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func TestJobCmd(t *testing.T) {
	// Mock Orchestrator
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/jobs":
			if r.Method == http.MethodGet {
				// List
				jobs := []orchestrator.JobInfo{
					{
						ID:        "job-1",
						Summary:   "Test Job",
						Status:    "Running",
						StartTime: time.Now(),
					},
				}
				json.NewEncoder(w).Encode(jobs)
			} else if r.Method == http.MethodPost {
				// Submit
				var item orchestrator.WorkItem
				json.NewDecoder(r.Body).Decode(&item)
				if item.ID == "job-new" {
					w.WriteHeader(http.StatusAccepted)
				} else {
					w.WriteHeader(http.StatusBadRequest)
				}
			}
		case "/jobs/job-1":
			if r.Method == http.MethodGet {
				job := orchestrator.JobInfo{
					ID:        "job-1",
					Summary:   "Test Job",
					Status:    "Running",
					StartTime: time.Now(),
				}
				json.NewEncoder(w).Encode(job)
			} else if r.Method == http.MethodDelete {
				w.WriteHeader(http.StatusOK)
			}
		case "/jobs/job-1/logs":
			fmt.Fprint(w, "Log line 1\nLog line 2")
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	// Override host via flag in args
	hostFlag := fmt.Sprintf("--host=%s", ts.URL)

	// Test List
	t.Run("List", func(t *testing.T) {
		output, err := executeJobCommand("list", hostFlag)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !strings.Contains(output, "job-1") {
			t.Errorf("Expected output to contain 'job-1', got: %s", output)
		}
	})

	// Test Info
	t.Run("Info", func(t *testing.T) {
		output, err := executeJobCommand("info", "job-1", hostFlag)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !strings.Contains(output, "Test Job") {
			t.Errorf("Expected output to contain 'Test Job', got: %s", output)
		}
	})

	// Test Logs
	t.Run("Logs", func(t *testing.T) {
		output, err := executeJobCommand("logs", "job-1", hostFlag)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !strings.Contains(output, "Log line 1") {
			t.Errorf("Expected output to contain 'Log line 1', got: %s", output)
		}
	})

	// Test Cancel
	t.Run("Cancel", func(t *testing.T) {
		output, err := executeJobCommand("cancel", "job-1", hostFlag)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !strings.Contains(output, "cancellation requested") {
			t.Errorf("Expected output to contain 'cancellation requested', got: %s", output)
		}
	})

	// Test Submit
	t.Run("Submit", func(t *testing.T) {
		output, err := executeJobCommand("submit", "--id", "job-new", "--summary", "New Job", "--repo-url", "http://git.com", hostFlag)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !strings.Contains(output, "submitted successfully") {
			t.Errorf("Expected output to contain 'submitted successfully', got: %s", output)
		}
	})

	// Test Monitor (Mocked)
	t.Run("Monitor", func(t *testing.T) {
		called := false
		startDashboardFunc = func(host string) error {
			called = true
			if host != ts.URL { // hostURL in test body was ts.URL, checking against that
				t.Errorf("Expected host %s, got %s", ts.URL, host)
			}
			return nil
		}
		defer func() { startDashboardFunc = tui.StartDashboard }()

		_, err := executeJobCommand("monitor", hostFlag)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !called {
			t.Error("Expected startDashboardFunc to be called")
		}
	})
}
