package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCancelJobsOlderThan_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs", r.URL.Path)
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "24h", r.URL.Query().Get("older_than"))

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"canceled": 5}`))
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

	cancelJobsOlderThan(server.URL, "24h")

	assert.False(t, exitCalled)
	assert.Contains(t, buf.String(), "Successfully canceled 5 jobs older than '24h'.")
}

func TestCancelJobsOlderThan_APIError(t *testing.T) {
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

	cancelJobsOlderThan(server.URL, "invalid")

	assert.True(t, exitCalled)
	assert.Contains(t, buf.String(), "Failed to cancel jobs older than invalid: invalid duration")
}
