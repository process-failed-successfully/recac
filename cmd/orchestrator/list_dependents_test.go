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

func TestListDependents_Success(t *testing.T) {
	now := time.Now()
	// Mock jobs
	jobs := []orchestrator.JobInfo{
		{
			ID:        "job-1",
			Summary:   "Dependent Job 1",
			Status:    "Running",
			StartTime: now.Add(-5 * time.Minute),
		},
		{
			ID:        "job-2",
			Summary:   "Dependent Job 2",
			Status:    "Pending",
			StartTime: now.Add(-2 * time.Minute),
			EndTime:   now,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs/target-job/dependents" {
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

	listDependents(server.URL, "target-job")

	assert.False(t, exitCalled, "listDependents should not call exit on success")

	output := buf.String()
	assert.Contains(t, output, "Dependents of target-job (2)")
	assert.Contains(t, output, "job-1")
	assert.Contains(t, output, "Dependent Job 1")
	assert.Contains(t, output, "Running")
	assert.Contains(t, output, "job-2")
	assert.Contains(t, output, "Dependent Job 2")
	assert.Contains(t, output, "Pending")
}

func TestListDependents_NoDependents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs/target-job/dependents" {
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

	listDependents(server.URL, "target-job")

	assert.False(t, exitCalled, "listDependents should not call exit on success")

	output := buf.String()
	assert.Contains(t, output, "No dependents found for job target-job.")
}

func TestListDependents_NotFound(t *testing.T) {
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

	listDependents(server.URL, "target-job")

	assert.Equal(t, 1, exitCode, "listDependents should exit with code 1 when API returns an error")

	output := buf.String()
	assert.Contains(t, output, "Failed to fetch dependents: Job not found")
}

func TestListDependents_ConnectionError(t *testing.T) {
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

	listDependents("http://invalid-url-1234567.com", "target-job")

	assert.Equal(t, 1, exitCode, "listDependents should exit with code 1 on connection error")
	output := buf.String()
	assert.Contains(t, output, "Failed to connect to orchestrator")
}

func TestListDependents_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs/target-job/dependents" {
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

	listDependents(server.URL, "target-job")

	assert.Equal(t, 1, exitCode, "listDependents should exit with code 1 on invalid JSON")

	output := buf.String()
	assert.Contains(t, output, "Failed to decode response")
}
