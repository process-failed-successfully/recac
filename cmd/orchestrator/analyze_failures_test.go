package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAnalyzeFailures(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "all", r.URL.Query().Get("state"))
		assert.Equal(t, "Failed", r.URL.Query().Get("status"))

		// Return a mix of failed jobs
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[
			{"id": "job-1", "summary": "timeout exceeded", "status": "Failed"},
			{"id": "job-2", "summary": "timeout exceeded", "status": "Failed"},
			{"id": "job-3", "summary": "timeout exceeded", "status": "Failed"},
			{"id": "job-4", "summary": "connection refused", "status": "Failed"},
			{"id": "job-5", "summary": "connection refused", "status": "Failed"},
			{"id": "job-6", "summary": "out of memory", "status": "Failed"},
			{"id": "job-7", "summary": "", "status": "Failed"}
		]`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	analyzeFailures(server.URL)

	assert.Equal(t, 0, exitCode)
	output := out.String()

	// Verify headers and content
	assert.Contains(t, output, "Failed Jobs Analysis (7 total)")
	assert.Contains(t, output, "Count")
	assert.Contains(t, output, "Error Signature (Summary)")
	assert.Contains(t, output, "Job IDs")

	// Verify groups
	// timeout exceeded count = 3
	assert.Contains(t, output, "3")
	assert.Contains(t, output, "timeout exceeded")
	assert.Contains(t, output, "job-1, job-2, job-3")

	// connection refused count = 2
	assert.Contains(t, output, "2")
	assert.Contains(t, output, "connection refused")
	assert.Contains(t, output, "job-4, job-5")

	// out of memory count = 1
	assert.Contains(t, output, "1")
	assert.Contains(t, output, "out of memory")
	assert.Contains(t, output, "job-6")

	// empty summary count = 1
	assert.Contains(t, output, "<empty summary>")
	assert.Contains(t, output, "job-7")
}

func TestAnalyzeFailures_NoJobs(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	analyzeFailures(server.URL)

	assert.Equal(t, 0, exitCode)
	output := out.String()
	assert.Contains(t, output, "No failed jobs found.")
}

func TestAnalyzeFailures_ServerError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`internal server error`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	analyzeFailures(server.URL)

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to fetch failed jobs")
	assert.Contains(t, out.String(), "internal server error")
}

func TestAnalyzeFailures_ConnectionError(t *testing.T) {
	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	analyzeFailures("http://invalid-host:12345")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to connect to orchestrator")
}
