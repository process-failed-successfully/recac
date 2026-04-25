package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetLogs_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs/test-job/logs", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("log line 1\nlog line 2"))
	}))
	defer server.Close()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	getLogs(server.URL, "test-job")

	out := buf.String()
	assert.Contains(t, out, "log line 1\nlog line 2")
}

func TestGetLogs_ConnectionError(t *testing.T) {
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	exitCalled := false
	oldExit := exitFunc
	exitFunc = func(code int) { exitCalled = true }
	defer func() { exitFunc = oldExit }()

	getLogs("http://invalid-host", "test-job")

	out := buf.String()
	assert.Contains(t, out, "Failed to connect to orchestrator")
	assert.True(t, exitCalled)
}

func TestGetLogs_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`job not found`))
	}))
	defer server.Close()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	exitCalled := false
	oldExit := exitFunc
	exitFunc = func(code int) { exitCalled = true }
	defer func() { exitFunc = oldExit }()

	getLogs(server.URL, "test-job")

	out := buf.String()
	assert.Contains(t, out, "Failed to fetch logs: job not found")
	assert.True(t, exitCalled)
}
