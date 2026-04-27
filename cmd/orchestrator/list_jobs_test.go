package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestListJobs_MatchFilter(t *testing.T) {
	// Start a mock server to intercept the API request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs", r.URL.Path)

		// Assert that the 'match' query parameter was passed correctly
		matchParam := r.URL.Query().Get("match")
		assert.Equal(t, "timeout", matchParam)

		// Assert other parameters
		assert.Equal(t, "all", r.URL.Query().Get("state"))
		assert.Equal(t, "Failed", r.URL.Query().Get("status"))
		assert.Equal(t, "urgent", r.URL.Query().Get("tag"))

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id": "job-1", "summary": "Failed due to timeout", "status": "Failed"}]`))
	}))
	defer server.Close()

	// Intercept stdout to prevent spamming test output
	oldStdout := stdout
	r, w, _ := os.Pipe()
	stdout = w
	defer func() { stdout = oldStdout }()

	// Execute listJobs with the match filter
	listJobs(server.URL, true, "Failed", "urgent", "timeout", "", "table")

	// Read and verify stdout
	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	assert.Contains(t, output, "job-1")
	assert.Contains(t, output, "Failed due to timeout")
}

func TestListJobs_FormatJSON(t *testing.T) {
	// Start a mock server to intercept the API request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id": "job-json-1", "summary": "JSON Test", "status": "Running"}]`))
	}))
	defer server.Close()

	// Intercept stdout to prevent spamming test output
	oldStdout := stdout
	r, w, _ := os.Pipe()
	stdout = w
	defer func() { stdout = oldStdout }()

	// Execute listJobs with JSON format
	listJobs(server.URL, false, "", "", "", "", "json")

	// Read and verify stdout
	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	assert.Contains(t, output, `"id": "job-json-1"`)
	assert.Contains(t, output, `"summary": "JSON Test"`)
	// Ensure table headers are NOT present
	assert.NotContains(t, output, "Active Jobs")
}

func TestListPendingJobs_FormatJSON(t *testing.T) {
	// Start a mock server to intercept the API request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs", r.URL.Path)
		assert.Equal(t, "pending", r.URL.Query().Get("state"))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id": "job-pending-json", "summary": "Pending JSON Test", "status": "Pending"}]`))
	}))
	defer server.Close()

	// Intercept stdout to prevent spamming test output
	oldStdout := stdout
	r, w, _ := os.Pipe()
	stdout = w
	defer func() { stdout = oldStdout }()

	// Execute listPendingJobs with JSON format
	listPendingJobs(server.URL, "", "json")

	// Read and verify stdout
	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	assert.Contains(t, output, `"id": "job-pending-json"`)
	assert.Contains(t, output, `"summary": "Pending JSON Test"`)
	// Ensure table headers are NOT present
	assert.NotContains(t, output, "Pending Jobs")
}

func TestListJobs_FormatCSV(t *testing.T) {
	// Start a mock server to intercept the API request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id": "job-csv-1", "summary": "CSV Test", "status": "Running", "work_item": {"priority": 1, "tags": ["tag1", "tag2"]}}]`))
	}))
	defer server.Close()

	// Intercept stdout to prevent spamming test output
	oldStdout := stdout
	r, w, _ := os.Pipe()
	stdout = w
	defer func() { stdout = oldStdout }()

	// Execute listJobs with CSV format
	listJobs(server.URL, false, "", "", "", "", "csv")

	// Read and verify stdout
	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	assert.Contains(t, output, "ID,Summary,Status,Priority,Tags,Duration")
	assert.Contains(t, output, "job-csv-1,CSV Test,Running,1,\"tag1,tag2\",")
}

func TestListPendingJobs_FormatCSV(t *testing.T) {
	// Start a mock server to intercept the API request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs", r.URL.Path)
		assert.Equal(t, "pending", r.URL.Query().Get("state"))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id": "job-pending-csv", "summary": "Pending CSV Test", "status": "Pending", "work_item": {"priority": 2, "tags": ["tag3"]}}]`))
	}))
	defer server.Close()

	// Intercept stdout to prevent spamming test output
	oldStdout := stdout
	r, w, _ := os.Pipe()
	stdout = w
	defer func() { stdout = oldStdout }()

	// Execute listPendingJobs with CSV format
	listPendingJobs(server.URL, "", "csv")

	// Read and verify stdout
	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	assert.Contains(t, output, "ID,Summary,Status,Priority,Tags")
	assert.Contains(t, output, "job-pending-csv,Pending CSV Test,Pending,2,tag3")
}

func TestListJobs_PriorityFilter(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "10", r.URL.Query().Get("priority"))

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[
			{"id": "job-p1", "summary": "High Priority Job", "status": "Running", "work_item": {"priority": 10}}
		]`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	r, w, _ := os.Pipe()
	oldStdout := stdout
	stdout = w
	defer func() { stdout = oldStdout }()

	listJobs(server.URL, false, "", "", "", "10", "table")

	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	assert.Contains(t, output, "job-p1")
	assert.Contains(t, output, "High Priority Job")
}
