package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"recac/internal/orchestrator"
)

func TestJobCmd(t *testing.T) {
	// Mock Orchestrator Server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/jobs":
			if r.Method == http.MethodGet {
				// List
				jobs := []orchestrator.JobInfo{
					{
						ID:        "JOB-1",
						Summary:   "Test Job",
						Status:    "Running",
						StartTime: time.Now(),
						WorkItem: orchestrator.WorkItem{
							RepoURL: "https://github.com/example/repo",
						},
					},
				}
				json.NewEncoder(w).Encode(jobs)
			} else if r.Method == http.MethodPost {
				// Submit
				var item orchestrator.WorkItem
				if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				if item.Summary == "" {
					http.Error(w, "Summary required", http.StatusBadRequest)
					return
				}
				w.WriteHeader(http.StatusAccepted)
			}
		case strings.HasPrefix(r.URL.Path, "/jobs/"):
			id := strings.TrimPrefix(r.URL.Path, "/jobs/")
			if strings.HasSuffix(id, "/logs") {
				// Logs
				w.Write([]byte("Log line 1\nLog line 2"))
				return
			}
			if id == "INVALID" {
				http.Error(w, "Not Found", http.StatusNotFound)
				return
			}
			if r.Method == http.MethodGet {
				// Info
				job := orchestrator.JobInfo{
					ID:        id,
					Summary:   "Test Job",
					Status:    "Running",
					StartTime: time.Now(),
					WorkItem: orchestrator.WorkItem{
						RepoURL: "https://github.com/example/repo",
						Description: "Test Description",
					},
				}
				json.NewEncoder(w).Encode(job)
			} else if r.Method == http.MethodDelete {
				// Cancel
				w.WriteHeader(http.StatusOK)
			}
		default:
			http.Error(w, "Not Found", http.StatusNotFound)
		}
	}))
	defer ts.Close()

	// Capture output
	buf := new(bytes.Buffer)

	// Test List
	t.Run("List", func(t *testing.T) {
		buf.Reset()
		rootCmd.SetOut(buf)
		rootCmd.SetArgs([]string{"job", "list", "--host", ts.URL})
		if err := rootCmd.Execute(); err != nil {
			t.Errorf("List failed: %v", err)
		}
		if !strings.Contains(buf.String(), "JOB-1") {
			t.Errorf("List output missing job ID. Got: %s", buf.String())
		}
	})

	// Test Info
	t.Run("Info", func(t *testing.T) {
		buf.Reset()
		rootCmd.SetOut(buf)
		rootCmd.SetArgs([]string{"job", "info", "JOB-1", "--host", ts.URL})
		if err := rootCmd.Execute(); err != nil {
			t.Errorf("Info failed: %v", err)
		}
		if !strings.Contains(buf.String(), "Test Job") {
			t.Errorf("Info output missing summary. Got: %s", buf.String())
		}
	})

	// Test Submit
	t.Run("Submit", func(t *testing.T) {
		buf.Reset()
		rootCmd.SetOut(buf)
		rootCmd.SetArgs([]string{"job", "submit", "--task", "Fix bugs", "--repo", "http://repo", "--id", "JOB-NEW", "--host", ts.URL})
		if err := rootCmd.Execute(); err != nil {
			t.Errorf("Submit failed: %v", err)
		}
		if !strings.Contains(buf.String(), "submitted successfully") {
			t.Errorf("Submit output missing success message. Got: %s", buf.String())
		}
	})

	// Test Cancel
	t.Run("Cancel", func(t *testing.T) {
		buf.Reset()
		rootCmd.SetOut(buf)
		rootCmd.SetArgs([]string{"job", "cancel", "JOB-1", "--host", ts.URL})
		if err := rootCmd.Execute(); err != nil {
			t.Errorf("Cancel failed: %v", err)
		}
		if !strings.Contains(buf.String(), "cancelled") {
			t.Errorf("Cancel output missing confirmation. Got: %s", buf.String())
		}
	})

	// Test Error
	t.Run("Error", func(t *testing.T) {
		rootCmd.SetOut(io.Discard)
		rootCmd.SetArgs([]string{"job", "info", "INVALID", "--host", ts.URL})
		if err := rootCmd.Execute(); err == nil {
			t.Errorf("Expected error for invalid job")
		}
	})
}
