package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"recac/internal/orchestrator"
)

func TestSearchJobsGlobally_Success(t *testing.T) {
	mockJobs := []orchestrator.JobInfo{
		{
			ID:      "JOB-1",
			Status:  "Completed",
			Summary: "Fix authentication issue",
			WorkItem: orchestrator.WorkItem{
				Tags: []string{"backend"},
			},
		},
		{
			ID:      "JOB-2",
			Status:  "Failed",
			Summary: "Update user interface layout with a very long summary that exceeds 50 characters",
			WorkItem: orchestrator.WorkItem{
				Tags: []string{"frontend", "urgent"},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs/search", r.URL.Path)
		assert.Equal(t, "fix|update", r.URL.Query().Get("q"))
		assert.Equal(t, "frontend", r.URL.Query().Get("tag"))
		assert.Equal(t, "Failed", r.URL.Query().Get("status"))

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(mockJobs)
	}))
	defer server.Close()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	exitCalled := false
	oldExit := exitFunc
	exitFunc = func(code int) {
		exitCalled = true
	}
	defer func() { exitFunc = oldExit }()

	searchJobsGlobally(server.URL, "fix|update", "frontend", "Failed", "table")

	assert.False(t, exitCalled)
	output := buf.String()
	assert.Contains(t, output, "ID")
	assert.Contains(t, output, "STATUS")
	assert.Contains(t, output, "SUMMARY")
	assert.Contains(t, output, "TAGS")
	assert.Contains(t, output, "JOB-1")
	assert.Contains(t, output, "JOB-2")
	assert.Contains(t, output, "backend")
	assert.Contains(t, output, "frontend, urgent")
	assert.Contains(t, output, "Update user interface layout with a very long s...") // truncated
}

func TestSearchJobsGlobally_JSONFormat(t *testing.T) {
	mockJobs := []orchestrator.JobInfo{
		{
			ID:      "JOB-1",
			Status:  "Completed",
			Summary: "Fix authentication issue",
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(mockJobs)
	}))
	defer server.Close()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	searchJobsGlobally(server.URL, "fix", "", "", "json")

	output := buf.String()
	assert.Contains(t, output, `"id": "JOB-1"`)
}

func TestSearchJobsGlobally_EmptyResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode([]orchestrator.JobInfo{})
	}))
	defer server.Close()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	searchJobsGlobally(server.URL, "nothing", "", "", "table")

	assert.Contains(t, buf.String(), "No matching jobs found.")
}

func TestSearchJobsGlobally_ConnectionError(t *testing.T) {
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	exitCalled := false
	oldExit := exitFunc
	exitFunc = func(code int) {
		exitCalled = true
	}
	defer func() { exitFunc = oldExit }()

	searchJobsGlobally("http://localhost:12345", "test", "", "", "table")

	assert.True(t, exitCalled)
	assert.Contains(t, buf.String(), "Failed to connect to orchestrator")
}

func TestSearchJobsGlobally_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid regex"))
	}))
	defer server.Close()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	exitCalled := false
	oldExit := exitFunc
	exitFunc = func(code int) {
		exitCalled = true
	}
	defer func() { exitFunc = oldExit }()

	searchJobsGlobally(server.URL, "[invalid", "", "", "table")

	assert.True(t, exitCalled)
	assert.Contains(t, buf.String(), "Failed to search jobs: invalid regex")
}
