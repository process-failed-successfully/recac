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

func executeRootCommand(args ...string) (string, error) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
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

	// Override host via flag in args, or viper
	// Since we use rootCmd, we can pass --host flag.
	hostFlag := fmt.Sprintf("--host=%s", ts.URL)
	// Remove protocol for check
	hostURL := ts.URL

	// Test List
	t.Run("List", func(t *testing.T) {
		output, err := executeRootCommand("job", "list", hostFlag)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !strings.Contains(output, "job-1") {
			t.Errorf("Expected output to contain 'job-1', got: %s", output)
		}
	})

	// Test Info
	t.Run("Info", func(t *testing.T) {
		output, err := executeRootCommand("job", "info", "job-1", hostFlag)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !strings.Contains(output, "Test Job") {
			t.Errorf("Expected output to contain 'Test Job', got: %s", output)
		}
	})

	// Test Logs
	t.Run("Logs", func(t *testing.T) {
		output, err := executeRootCommand("job", "logs", "job-1", hostFlag)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !strings.Contains(output, "Log line 1") {
			t.Errorf("Expected output to contain 'Log line 1', got: %s", output)
		}
	})

	// Test Cancel
	t.Run("Cancel", func(t *testing.T) {
		output, err := executeRootCommand("job", "cancel", "job-1", hostFlag)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !strings.Contains(output, "cancellation requested") {
			t.Errorf("Expected output to contain 'cancellation requested', got: %s", output)
		}
	})

	// Test Submit
	t.Run("Submit", func(t *testing.T) {
		output, err := executeRootCommand("job", "submit", "--id", "job-new", "--summary", "New Job", "--repo-url", "http://git.com", hostFlag)
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
			if host != hostURL {
				t.Errorf("Expected host %s, got %s", hostURL, host)
			}
			return nil
		}
		defer func() { startDashboardFunc = tui.StartDashboard }()

		_, err := executeRootCommand("job", "monitor", hostFlag)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !called {
			t.Error("Expected startDashboardFunc to be called")
		}
	})
}
