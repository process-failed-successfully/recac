package tui

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"recac/internal/orchestrator"

	"github.com/stretchr/testify/assert"
)

func TestCmd_FetchStatus(t *testing.T) {
	// Mock Server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/status" {
			status := orchestrator.Status{Uptime: "1h"}
			json.NewEncoder(w).Encode(status)
			return
		}
		if r.URL.Path == "/jobs" {
			if r.URL.Query().Get("state") == "all" {
				// History
			}
			jobs := []orchestrator.JobInfo{{ID: "JOB-1"}}
			json.NewEncoder(w).Encode(jobs)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	// Test fetchStatus
	cmd := fetchStatus(server.URL, false)
	msg := cmd()

	sMsg, ok := msg.(statusMsg)
	assert.True(t, ok)
	assert.Nil(t, sMsg.Err)
	assert.Equal(t, "1h", sMsg.Status.Uptime)
	assert.Len(t, sMsg.Jobs, 1)
	assert.Equal(t, "JOB-1", sMsg.Jobs[0].ID)
}

func TestCmd_FetchStatus_Error(t *testing.T) {
	// Closed server
	server := httptest.NewServer(http.HandlerFunc(nil))
	server.Close()

	cmd := fetchStatus(server.URL, false)
	msg := cmd()

	sMsg, ok := msg.(statusMsg)
	assert.True(t, ok)
	assert.NotNil(t, sMsg.Err)
}

func TestCmd_FetchJobDetails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs/JOB-1" {
			job := orchestrator.JobInfo{ID: "JOB-1", Summary: "Details"}
			json.NewEncoder(w).Encode(job)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	cmd := fetchJobDetails(server.URL, "JOB-1")
	msg := cmd()

	dMsg, ok := msg.(detailsMsg)
	assert.True(t, ok)
	assert.Nil(t, dMsg.Err)
	assert.Equal(t, "JOB-1", dMsg.Job.ID)
	assert.Equal(t, "Details", dMsg.Job.Summary)
}

func TestCmd_FetchJobDetails_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	cmd := fetchJobDetails(server.URL, "JOB-1")
	msg := cmd()

	dMsg, ok := msg.(detailsMsg)
	assert.True(t, ok)
	assert.NotNil(t, dMsg.Err)
	assert.Contains(t, dMsg.Err.Error(), "status 404")
}

func TestCmd_FetchJobLogs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs/JOB-1/logs" {
			w.Write([]byte("some logs"))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	cmd := streamJobLogs(server.URL, "JOB-1")
	msg := cmd()

	lMsg, ok := msg.(logStreamMsg)
	assert.True(t, ok)
	assert.Nil(t, lMsg.Err)

	defer lMsg.Stream.Close()
	content, err := io.ReadAll(lMsg.Stream)
	assert.NoError(t, err)
	assert.Equal(t, "some logs", string(content))
}

func TestCmd_FetchJobLogs_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Error", http.StatusInternalServerError)
	}))
	defer server.Close()

	cmd := streamJobLogs(server.URL, "JOB-1")
	msg := cmd()

	lMsg, ok := msg.(logStreamMsg)
	assert.True(t, ok)
	assert.NotNil(t, lMsg.Err)
	assert.Contains(t, lMsg.Err.Error(), "status 500")
}

func TestCmd_CancelJob(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete && r.URL.Path == "/jobs/JOB-1" {
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	cmd := cancelJob(server.URL, "JOB-1")
	msg := cmd()

	aMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Nil(t, aMsg.Err)
	assert.Equal(t, "Cancelled", aMsg.Message)
}

func TestCmd_RetryJob(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/jobs/JOB-1/retry" {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	cmd := retryJob(server.URL, "JOB-1")
	msg := cmd()

	aMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Nil(t, aMsg.Err)
	assert.Equal(t, "Retried", aMsg.Message)
}

func TestRenderDetails(t *testing.T) {
	job := orchestrator.JobInfo{
		ID:        "JOB-1",
		Summary:   "Summary",
		Status:    "Running",
		StartTime: time.Now(),
		WorkItem: orchestrator.WorkItem{
			RepoURL:     "http://repo",
			Description: "Desc",
			EnvVars:     map[string]string{"KEY": "VAL"},
		},
	}

	output := renderDetails(job)
	assert.Contains(t, output, "JOB-1")
	assert.Contains(t, output, "Summary")
	assert.Contains(t, output, "Running")
	assert.Contains(t, output, "KEY=VAL")
}
