package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSkipJobsOlderThan_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs/skip", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "24h", r.URL.Query().Get("older_than"))

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"skipped": 5}`))
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

	skipJobsOlderThan(server.URL, "24h")

	assert.False(t, exitCalled)
	assert.Contains(t, buf.String(), "Successfully skipped 5 jobs older than '24h'.")
}

func TestSkipJobsOlderThan_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid duration"))
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

	skipJobsOlderThan(server.URL, "invalid")

	assert.True(t, exitCalled)
	assert.Contains(t, buf.String(), "Failed to skip jobs older than invalid: invalid duration")
}

func TestSkipJobsOlderThan_Errors(t *testing.T) {
	origExit := exitFunc
	defer func() { exitFunc = origExit }()
	var exitCode int
	exitFunc = func(code int) { exitCode = code }

	origStdout := stdout
	defer func() { stdout = origStdout }()

	t.Run("InvalidURL", func(t *testing.T) {
		var buf bytes.Buffer
		stdout = &buf
		exitCode = 0

		skipJobsOlderThan("http://\x00invalid", "24h")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to parse URL")
	})

	t.Run("ConnectionError", func(t *testing.T) {
		var buf bytes.Buffer
		stdout = &buf
		exitCode = 0

		skipJobsOlderThan("http://localhost:12345", "24h")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to connect to orchestrator")
	})

	t.Run("BadJSONResponse", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{bad json}`))
		}))
		defer server.Close()

		var buf bytes.Buffer
		stdout = &buf
		exitCode = 0

		skipJobsOlderThan(server.URL, "24h")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to decode response")
	})

	t.Run("UnexpectedResponseFormat", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"other_field": 123}`))
		}))
		defer server.Close()

		var buf bytes.Buffer
		stdout = &buf
		exitCode = 0

		skipJobsOlderThan(server.URL, "24h")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Unexpected response format")
	})
}
