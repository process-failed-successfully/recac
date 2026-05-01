package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRetryFailedJobs_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/jobs/retry-failed", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"retried": 5}`))
	}))
	defer server.Close()

	oldStdout := stdout
	r, w, _ := os.Pipe()
	stdout = w
	defer func() { stdout = oldStdout }()

	retryFailedJobs(server.URL, "", "", "")

	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	assert.Contains(t, buf.String(), "Successfully retried 5 failed jobs.")
}

func TestRetryFailedJobs_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Error"))
	}))
	defer server.Close()

	oldExit := exitFunc
	var exitCode int
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	oldStdout := stdout
	r, w, _ := os.Pipe()
	stdout = w
	defer func() { stdout = oldStdout }()

	retryFailedJobs(server.URL, "", "", "")

	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	assert.Contains(t, buf.String(), "Failed to retry failed jobs")
	assert.Equal(t, 1, exitCode)
}

func TestRetryFailedJobs_ConnectionError(t *testing.T) {
	oldExit := exitFunc
	var exitCode int
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	oldStdout := stdout
	r, w, _ := os.Pipe()
	stdout = w
	defer func() { stdout = oldStdout }()

	retryFailedJobs("http://invalid-url-that-will-fail.test", "", "", "")

	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	assert.Contains(t, buf.String(), "Failed to connect to orchestrator")
	assert.Equal(t, 1, exitCode)
}

func TestRetryFailedJobs_InvalidResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	oldExit := exitFunc
	var exitCode int
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	oldStdout := stdout
	r, w, _ := os.Pipe()
	stdout = w
	defer func() { stdout = oldStdout }()

	retryFailedJobs(server.URL, "", "", "")

	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	assert.Contains(t, buf.String(), "Failed to decode response")
	assert.Equal(t, 1, exitCode)
}

func TestRetryFailedJobs_InvalidURL(t *testing.T) {
	oldExit := exitFunc
	var exitCode int
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	oldStdout := stdout
	r, w, _ := os.Pipe()
	stdout = w
	defer func() { stdout = oldStdout }()

	retryFailedJobs("http://[fe80::1%en0]:8080", "", "", "")

	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	assert.Contains(t, buf.String(), "Failed to parse URL")
	assert.Equal(t, 1, exitCode)
}

func TestRetryFailedJobs_WithMatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "match_string", r.URL.Query().Get("match"))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"retried": 2}`))
	}))
	defer server.Close()

	oldStdout := stdout
	r, w, _ := os.Pipe()
	stdout = w
	defer func() { stdout = oldStdout }()

	retryFailedJobs(server.URL, "match_string", "", "")

	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	assert.Contains(t, buf.String(), "Successfully retried 2 failed jobs.")
}

func TestRetryFailedJobs_WithTag(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "backend", r.URL.Query().Get("tag"))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"retried": 3}`))
	}))
	defer server.Close()

	oldStdout := stdout
	r, w, _ := os.Pipe()
	stdout = w
	defer func() { stdout = oldStdout }()

	retryFailedJobs(server.URL, "", "backend", "")

	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	assert.Contains(t, buf.String(), "Successfully retried 3 failed jobs.")
}

func TestRetryFailedJobs_WithGroup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "data", r.URL.Query().Get("group"))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"retried": 4}`))
	}))
	defer server.Close()

	oldStdout := stdout
	r, w, _ := os.Pipe()
	stdout = w
	defer func() { stdout = oldStdout }()

	retryFailedJobs(server.URL, "", "", "data")

	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	assert.Contains(t, buf.String(), "Successfully retried 4 failed jobs.")
}
