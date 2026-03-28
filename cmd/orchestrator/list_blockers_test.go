package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"recac/internal/orchestrator"

	"github.com/stretchr/testify/assert"
)

func TestListBlockers_Success(t *testing.T) {
	now := time.Now()
	// Mock jobs
	jobs := []orchestrator.JobInfo{
		{
			ID:        "job-1",
			Summary:   "Blocking Job 1",
			Status:    "Running",
			StartTime: now.Add(-5 * time.Minute),
		},
		{
			ID:        "job-2",
			Summary:   "Blocking Job 2",
			Status:    "Pending",
			StartTime: now.Add(-2 * time.Minute),
			EndTime:   now,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs/target-job/blockers" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(jobs)
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

	listBlockers(server.URL, "target-job")

	assert.False(t, exitCalled, "listBlockers should not call exit on success")

	output := buf.String()
	assert.Contains(t, output, "Blockers of target-job (2)")
	assert.Contains(t, output, "job-1")
	assert.Contains(t, output, "Blocking Job 1")
	assert.Contains(t, output, "Running")
	assert.Contains(t, output, "job-2")
	assert.Contains(t, output, "Blocking Job 2")
	assert.Contains(t, output, "Pending")
}

func TestListBlockers_NoBlockers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs/target-job/blockers" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]orchestrator.JobInfo{})
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

	listBlockers(server.URL, "target-job")

	assert.False(t, exitCalled, "listBlockers should not call exit on success")

	output := buf.String()
	assert.Contains(t, output, "No blockers found for job target-job.")
}

func TestListBlockers_NotFound(t *testing.T) {
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

	listBlockers(server.URL, "target-job")

	assert.Equal(t, 1, exitCode, "listBlockers should exit with code 1 when API returns an error")

	output := buf.String()
	assert.Contains(t, output, "Failed to fetch blockers: Job not found")
}

func TestListBlockers_ConnectionError(t *testing.T) {
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

	listBlockers("http://invalid-url-1234567.com", "target-job")

	assert.Equal(t, 1, exitCode, "listBlockers should exit with code 1 on connection error")
	output := buf.String()
	assert.Contains(t, output, "Failed to connect to orchestrator")
}

func TestListBlockers_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs/target-job/blockers" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("invalid json"))
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
	exitCode := -1
	exitFunc = func(code int) {
		exitCode = code
	}
	defer func() { exitFunc = originalExitFunc }()

	listBlockers(server.URL, "target-job")

	assert.Equal(t, 1, exitCode, "listBlockers should exit with code 1 on invalid JSON")

	output := buf.String()
	assert.Contains(t, output, "Failed to decode response")
}