package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"recac/internal/orchestrator"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func setupJobTest(t *testing.T) (*httptest.Server, *cobra.Command, *bytes.Buffer) {
	// Mock Server
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			state := r.URL.Query().Get("state")
			jobs := []orchestrator.JobInfo{
				{
					ID:        "job-1",
					Summary:   "Test Job 1",
					Status:    "Running",
					StartTime: time.Now().Add(-1 * time.Minute),
				},
			}
			if state == "all" {
				jobs = append(jobs, orchestrator.JobInfo{
					ID:        "job-2",
					Summary:   "Test Job 2",
					Status:    "Completed",
					StartTime: time.Now().Add(-10 * time.Minute),
					EndTime:   time.Now().Add(-5 * time.Minute),
				})
			}
			json.NewEncoder(w).Encode(jobs)
		} else if r.Method == http.MethodPost {
			var item orchestrator.WorkItem
			json.NewDecoder(r.Body).Decode(&item)
			w.WriteHeader(http.StatusAccepted)
		}
	})

	mux.HandleFunc("/jobs/job-1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			job := orchestrator.JobInfo{
				ID:        "job-1",
				Summary:   "Test Job 1",
				Status:    "Running",
				StartTime: time.Now().Add(-1 * time.Minute),
				WorkItem: orchestrator.WorkItem{
					RepoURL:     "https://github.com/test/repo",
					Description: "Test Description",
				},
			}
			json.NewEncoder(w).Encode(job)
		} else if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusOK)
		}
	})

	mux.HandleFunc("/jobs/job-1/logs", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "Log line 1\nLog line 2")
	})

	mux.HandleFunc("/jobs/job-2/retry", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})

	mux.HandleFunc("/jobs/retry-failed", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"retried": 3}`)
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	// Command Setup
	buf := new(bytes.Buffer)
	jobCmd.SetOut(buf)
	jobCmd.SetErr(buf)

	// Force subcommands to use the same output
	for _, sub := range jobCmd.Commands() {
		sub.SetOut(buf)
		sub.SetErr(buf)
	}

	// Reset flags
	viper.Reset()
	jobCmd.PersistentFlags().Set("host", server.URL)
	viper.Set("job.host", server.URL)

	return server, jobCmd, buf
}

func TestJobCmd_List(t *testing.T) {
	_, _, buf := setupJobTest(t)

	// Use Run directly
	jobListCmd.SetOut(buf)
	jobListCmd.Run(jobListCmd, []string{})

	assert.Contains(t, buf.String(), "Active Jobs")
	assert.Contains(t, buf.String(), "job-1")
	assert.NotContains(t, buf.String(), "job-2")

	buf.Reset()
	viper.Set("job.history", true)
	jobListCmd.Run(jobListCmd, []string{})

	assert.Contains(t, buf.String(), "All Jobs")
	assert.Contains(t, buf.String(), "job-1")
	assert.Contains(t, buf.String(), "job-2")
}

func TestJobCmd_Info(t *testing.T) {
	_, _, buf := setupJobTest(t)

	jobInfoCmd.SetOut(buf)
	jobInfoCmd.Run(jobInfoCmd, []string{"job-1"})

	assert.Contains(t, buf.String(), "Job Details: job-1")
	assert.Contains(t, buf.String(), "Test Job 1")
	assert.Contains(t, buf.String(), "Test Description")
}

func TestJobCmd_Logs(t *testing.T) {
	_, _, buf := setupJobTest(t)

	jobLogsCmd.SetOut(buf)
	jobLogsCmd.Run(jobLogsCmd, []string{"job-1"})

	assert.Contains(t, buf.String(), "Log line 1")
	assert.Contains(t, buf.String(), "Log line 2")
}

func TestJobCmd_Cancel(t *testing.T) {
	_, _, buf := setupJobTest(t)

	jobCancelCmd.SetOut(buf)
	jobCancelCmd.Run(jobCancelCmd, []string{"job-1"})

	assert.Contains(t, buf.String(), "Job job-1 cancellation requested")
}

func TestJobCmd_Retry(t *testing.T) {
	_, _, buf := setupJobTest(t)

	jobRetryCmd.SetOut(buf)
	jobRetryCmd.Run(jobRetryCmd, []string{"job-2"})

	assert.Contains(t, buf.String(), "Job job-2 retry submitted successfully")
}

func TestJobCmd_RetryFailed(t *testing.T) {
	_, _, buf := setupJobTest(t)

	jobRetryFailedCmd.SetOut(buf)
	jobRetryFailedCmd.Run(jobRetryFailedCmd, []string{})

	assert.Contains(t, buf.String(), "Successfully retried 3 failed jobs")
}

func TestJobCmd_Submit(t *testing.T) {
	_, _, buf := setupJobTest(t)

	// Create a temp file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "job.json")
	err := os.WriteFile(tmpFile, []byte(`{"id": "job-3", "summary": "Test Job 3"}`), 0644)
	assert.NoError(t, err)

	jobSubmitCmd.SetOut(buf)
	jobSubmitCmd.Run(jobSubmitCmd, []string{tmpFile})

	assert.Contains(t, buf.String(), "Job job-3 submitted successfully")
}
