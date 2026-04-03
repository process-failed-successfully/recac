package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExplainJob_Success(t *testing.T) {
	// 1. Mock the API Server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs/TEST-123/explain" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"explanation": "### Analysis\nThe job failed due to a syntax error on line 42."})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// 2. Capture stdout
	var buf bytes.Buffer
	originalStdout := stdout
	stdout = &buf
	defer func() { stdout = originalStdout }()

	// Prevent exitFunc from actually exiting
	originalExitFunc := exitFunc
	exitCalled := false
	exitFunc = func(code int) {
		exitCalled = true
	}
	defer func() { exitFunc = originalExitFunc }()

	// 3. Run the function
	explainJob(server.URL, "TEST-123", "mock-provider", "mock-model")

	// 4. Verify expectations
	assert.False(t, exitCalled, "explainJob should not exit on success")

	output := buf.String()
	assert.Contains(t, output, "Fetching explanation for TEST-123...")
	assert.Contains(t, output, "Analysis")
	assert.Contains(t, output, "syntax error on line 42")
}

func TestExplainJob_JobNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Job not found"))
	}))
	defer server.Close()

	var buf bytes.Buffer
	originalStdout := stdout
	stdout = &buf
	defer func() { stdout = originalStdout }()

	originalExitFunc := exitFunc
	exitCode := -1
	exitFunc = func(code int) {
		exitCode = code
	}
	defer func() { exitFunc = originalExitFunc }()

	explainJob(server.URL, "MISSING-123", "mock-provider", "mock-model")

	assert.Equal(t, 1, exitCode, "explainJob should exit with code 1 when job is not found")
	output := buf.String()
	assert.Contains(t, output, "Failed to get explanation: Job not found")
}

func TestExplainJob_ConnectionError(t *testing.T) {
	var buf bytes.Buffer
	originalStdout := stdout
	stdout = &buf
	defer func() { stdout = originalStdout }()

	originalExitFunc := exitFunc
	exitCode := -1
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = originalExitFunc }()

	explainJob("http://invalid-url:12345", "TEST-123", "", "")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, buf.String(), "Failed to connect to orchestrator")
}

func TestExplainJob_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	var buf bytes.Buffer
	originalStdout := stdout
	stdout = &buf
	defer func() { stdout = originalStdout }()

	originalExitFunc := exitFunc
	exitCode := -1
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = originalExitFunc }()

	explainJob(server.URL, "TEST-123", "", "")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, buf.String(), "Failed to decode response:")
}

func TestExplainJob_AIFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Failed to initialize AI agent"))
	}))
	defer server.Close()

	var buf bytes.Buffer
	originalStdout := stdout
	stdout = &buf
	defer func() { stdout = originalStdout }()

	originalExitFunc := exitFunc
	exitCode := -1
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = originalExitFunc }()

	explainJob(server.URL, "TEST-123", "", "")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, buf.String(), "Failed to get explanation: Failed to initialize AI agent")
}

func TestExplainBulkJobs_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs/explain/bulk" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]map[string]string{
				"explanations": {
					"TEST-1": "Analysis for TEST-1",
					"TEST-2": "Analysis for TEST-2",
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	var buf bytes.Buffer
	originalStdout := stdout
	stdout = &buf
	defer func() { stdout = originalStdout }()

	originalExitFunc := exitFunc
	exitCalled := false
	exitFunc = func(code int) {
		exitCalled = true
	}
	defer func() { exitFunc = originalExitFunc }()

	explainBulkJobs(server.URL, "match-regex", "test-tag", "mock-provider", "mock-model")

	assert.False(t, exitCalled, "explainBulkJobs should not exit on success")

	output := buf.String()
	assert.Contains(t, output, "Fetching bulk explanations for failed jobs...")
	assert.Contains(t, output, "Job ID: TEST-1")
	assert.Contains(t, output, "Analysis for TEST-1")
	assert.Contains(t, output, "Job ID: TEST-2")
	assert.Contains(t, output, "Analysis for TEST-2")
}

func TestExplainBulkJobs_Empty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs/explain/bulk" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]map[string]string{
				"explanations": {},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	var buf bytes.Buffer
	originalStdout := stdout
	stdout = &buf
	defer func() { stdout = originalStdout }()

	originalExitFunc := exitFunc
	exitCalled := false
	exitFunc = func(code int) {
		exitCalled = true
	}
	defer func() { exitFunc = originalExitFunc }()

	explainBulkJobs(server.URL, "", "test-tag", "", "")

	assert.False(t, exitCalled, "explainBulkJobs should not exit when empty")

	output := buf.String()
	assert.Contains(t, output, "No failed jobs found matching the criteria.")
}
