package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDemoteJobCmd(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/JOB-1/demote", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"priority": 5}`))
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

	demoteJob(server.URL, "JOB-1")

	assert.Equal(t, 0, exitCode)
	assert.Contains(t, out.String(), "Successfully demoted job JOB-1 to priority 5")
}

func TestDemoteJobCmd_Error(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/JOB-1/demote", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`job not found`))
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

	demoteJob(server.URL, "JOB-1")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to demote job: job not found")
}

func TestDemoteJobCmd_RequestError(t *testing.T) {
	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	// Invalid URL to trigger Request Error
	demoteJob("http://192.168.0.%31", "JOB-1")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to create request")
}

func TestDemoteJobCmd_ConnectionError(t *testing.T) {
	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	// Valid URL but not running
	demoteJob("http://localhost:12345", "JOB-1")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to connect to orchestrator")
}

func TestDemoteJobCmd_DecodeError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/JOB-1/demote", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`invalid json`))
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

	demoteJob(server.URL, "JOB-1")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to decode response")
}

func TestDemoteBulkJobs_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/demote/bulk", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "tag1", r.URL.Query().Get("tag"))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"demoted": 5}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	demoteBulkJobs(server.URL, "", "tag1")

	assert.Contains(t, buf.String(), "Successfully demoted 5 jobs.")
}

func TestDemoteBulkJobs_Error(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/demote/bulk", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`internal server error`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	demoteBulkJobs(server.URL, "match-me", "")

	assert.Contains(t, buf.String(), "Failed to demote jobs: internal server error")
	assert.Equal(t, 1, exitCode)
}

func TestDemoteBulkJobs_ConnectionError(t *testing.T) {
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	demoteBulkJobs("http://localhost:0", "match-me", "")

	assert.Contains(t, buf.String(), "Failed to connect to orchestrator")
	assert.Equal(t, 1, exitCode)
}

func TestDemoteBulkJobs_DecodeError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/demote/bulk", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`invalid json`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	demoteBulkJobs(server.URL, "", "")

	assert.Contains(t, buf.String(), "Failed to decode response")
	assert.Equal(t, 1, exitCode)
}
