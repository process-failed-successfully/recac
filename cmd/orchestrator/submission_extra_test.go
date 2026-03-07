package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"recac/internal/orchestrator"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubmitJob_Success(t *testing.T) {
	// Mock Exit and Stdout
	var exitCode int
	originalExit := exitFunc
	exitFunc = func(code int) {
		exitCode = code
	}
	defer func() { exitFunc = originalExit }()

	var out bytes.Buffer
	originalStdout := stdout
	stdout = &out
	defer func() { stdout = originalStdout }()

	// Mock Server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs", r.URL.Path)
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte("Job submitted"))
	}))
	defer server.Close()

	// Create temp file
	tmpDir, err := os.MkdirTemp("", "submit-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "job.json")
	item := orchestrator.WorkItem{ID: "JOB-1", Summary: "Test Job"}
	data, _ := json.Marshal(item)
	os.WriteFile(filePath, data, 0644)

	// Execute
	submitJob(server.URL, filePath, false)

	// Verify
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, out.String(), "Job submitted")
}

func TestSubmitJob_FileNotFound(t *testing.T) {
	var exitCode int
	originalExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = originalExit }()

	var out bytes.Buffer
	originalStdout := stdout
	stdout = &out
	defer func() { stdout = originalStdout }()

	submitJob("http://localhost", "non-existent.json", false)

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to open file")
}

func TestSubmitJob_InvalidJSON(t *testing.T) {
	var exitCode int
	originalExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = originalExit }()

	var out bytes.Buffer
	originalStdout := stdout
	stdout = &out
	defer func() { stdout = originalStdout }()

	tmpDir, _ := os.MkdirTemp("", "submit-test-invalid")
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "invalid.json")
	os.WriteFile(filePath, []byte("invalid json"), 0644)

	submitJob("http://localhost", filePath, false)

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Invalid JSON")
}

func TestSubmitAdHocJob_ServerError(t *testing.T) {
	var exitCode int
	originalExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = originalExit }()

	var out bytes.Buffer
	originalStdout := stdout
	stdout = &out
	defer func() { stdout = originalStdout }()

	// Closed server
	server := httptest.NewServer(http.HandlerFunc(nil))
	server.Close()

	submitAdHocJob(server.URL, "http://repo.com", "Task", "ID", 0, false, nil, nil)

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to connect")
}

func TestSubmitAdHocJob_ErrorResponse(t *testing.T) {
	var exitCode int
	originalExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = originalExit }()

	var out bytes.Buffer
	originalStdout := stdout
	stdout = &out
	defer func() { stdout = originalStdout }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	submitAdHocJob(server.URL, "http://repo.com", "Task", "ID", 0, false, nil, nil)

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to submit job")
	assert.Contains(t, out.String(), "Internal Server Error")
}

func TestWaitForJob_Completed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs/JOB-1" {
			json.NewEncoder(w).Encode(orchestrator.JobInfo{Status: "Completed"})
		}
	}))
	defer server.Close()

	var out bytes.Buffer
	err := waitForJob(server.URL, "JOB-1", &out)

	assert.NoError(t, err)
	assert.Contains(t, out.String(), "Job already completed")
}

func TestSubmitJob_WithWait(t *testing.T) {
	// Mock Exit and Stdout
	var exitCode int
	originalExit := exitFunc
	exitFunc = func(code int) {
		exitCode = code
	}
	defer func() { exitFunc = originalExit }()

	var out bytes.Buffer
	originalStdout := stdout
	stdout = &out
	defer func() { stdout = originalStdout }()

	// Mock Server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusAccepted)
			w.Write([]byte("Job submitted"))
			return
		}
		if r.URL.Path == "/jobs/JOB-WAIT" {
			// Return completed immediately for simplicity
			json.NewEncoder(w).Encode(orchestrator.JobInfo{Status: "Completed"})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	// Create temp file
	tmpDir, err := os.MkdirTemp("", "submit-test-wait")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "job.json")
	// Use manual JSON to ensure lowercase "id" key as expected by submitJob
	data := []byte(`{"id": "JOB-WAIT", "summary": "Test Job"}`)
	os.WriteFile(filePath, data, 0644)

	// Execute
	submitJob(server.URL, filePath, true)

	// Verify
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, out.String(), "Job submitted")
	assert.Contains(t, out.String(), "Job already completed")
}

func TestSubmitAdHocJob_WithWait(t *testing.T) {
	// Mock Exit and Stdout
	var exitCode int
	originalExit := exitFunc
	exitFunc = func(code int) {
		exitCode = code
	}
	defer func() { exitFunc = originalExit }()

	var out bytes.Buffer
	originalStdout := stdout
	stdout = &out
	defer func() { stdout = originalStdout }()

	// Mock Server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusAccepted)
			w.Write([]byte("Job submitted"))
			return
		}
		if r.URL.Path == "/jobs/JOB-ADHOC-WAIT" {
			// Return completed immediately for simplicity
			json.NewEncoder(w).Encode(orchestrator.JobInfo{Status: "Completed"})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	// Execute
	submitAdHocJob(server.URL, "http://repo.com", "Task", "JOB-ADHOC-WAIT", 0, true, nil, nil)

	// Verify
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, out.String(), "Job submitted")
	assert.Contains(t, out.String(), "Job already completed")
}
