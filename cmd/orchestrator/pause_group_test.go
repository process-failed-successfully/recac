package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPauseOrchestratorGroupCmd(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/groups/test-group/pause", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		w.WriteHeader(http.StatusOK)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	pauseOrchestratorGroup(server.URL, "test-group")

	assert.Equal(t, 0, exitCode)
	assert.Contains(t, out.String(), "Concurrency group test-group paused")
}

func TestResumeOrchestratorGroupCmd(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/groups/test-group/resume", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		w.WriteHeader(http.StatusOK)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	resumeOrchestratorGroup(server.URL, "test-group")

	assert.Equal(t, 0, exitCode)
	assert.Contains(t, out.String(), "Concurrency group test-group resumed")
}

func TestPauseOrchestratorGroupCmd_ConnectionError(t *testing.T) {
	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	pauseOrchestratorGroup("http://invalid-host:12345", "test-group")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to connect to orchestrator")
}

func TestPauseOrchestratorGroupCmd_ErrorResponse(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/groups/test-group/pause", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`invalid request`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	pauseOrchestratorGroup(server.URL, "test-group")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to pause concurrency group")
}

func TestResumeOrchestratorGroupCmd_ConnectionError(t *testing.T) {
	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	resumeOrchestratorGroup("http://invalid-host:12345", "test-group")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to connect to orchestrator")
}

func TestResumeOrchestratorGroupCmd_ErrorResponse(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/groups/test-group/resume", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`invalid request`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	resumeOrchestratorGroup(server.URL, "test-group")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to resume concurrency group")
}

func TestPauseOrchestratorGroupCmd_InvalidURL(t *testing.T) {
	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	pauseOrchestratorGroup("::invalid-url", "test-group")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to create request")
}

func TestResumeOrchestratorGroupCmd_InvalidURL(t *testing.T) {
	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	resumeOrchestratorGroup("::invalid-url", "test-group")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to create request")
}
