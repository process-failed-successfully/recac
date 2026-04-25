package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPromoteJob_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs/test-job/promote", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"test-job", "priority":42}`))
	}))
	defer server.Close()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	promoteJob(server.URL, "test-job")

	out := buf.String()
	assert.Contains(t, out, "Successfully promoted job test-job to priority 42")
}

func TestPromoteJob_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`job missing not found in pending queue`))
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

	promoteJob(server.URL, "missing")

	out := buf.String()
	assert.Contains(t, out, "Failed to promote job: job missing not found in pending queue")
	assert.True(t, exitCalled)
}

func TestPromoteJob_ConnectionError(t *testing.T) {
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	exitCalled := false
	oldExit := exitFunc
	exitFunc = func(code int) { exitCalled = true }
	defer func() { exitFunc = oldExit }()

	promoteJob("http://invalid-host", "job-1")

	out := buf.String()
	assert.Contains(t, out, "Failed to connect to orchestrator")
	assert.True(t, exitCalled)
}

func TestPromoteJob_DecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`invalid json`))
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

	promoteJob(server.URL, "job-1")

	out := buf.String()
	assert.Contains(t, out, "Failed to decode response")
	assert.True(t, exitCalled)
}

func TestPromoteJob_InvalidHost(t *testing.T) {
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	exitCalled := false
	oldExit := exitFunc
	exitFunc = func(code int) { exitCalled = true }
	defer func() { exitFunc = oldExit }()

	promoteJob("::invalid_host", "job-1")

	out := buf.String()
	assert.Contains(t, out, "Failed to create request")
	assert.True(t, exitCalled)
}

func TestPromoteBulkJobs_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs/promote/bulk", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		q := r.URL.Query()
		assert.Equal(t, "m1", q.Get("match"))
		assert.Equal(t, "t1", q.Get("tag"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"promoted": 5}`))
	}))
	defer server.Close()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	promoteBulkJobs(server.URL, "m1", "t1")

	out := buf.String()
	assert.Contains(t, out, "Successfully promoted 5 jobs.")
}

func TestPromoteBulkJobs_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`bad request error`))
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

	promoteBulkJobs(server.URL, "", "")

	out := buf.String()
	assert.Contains(t, out, "Failed to promote jobs: bad request error")
	assert.True(t, exitCalled)
}

func TestPromoteBulkJobs_ConnectionError(t *testing.T) {
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	exitCalled := false
	oldExit := exitFunc
	exitFunc = func(code int) { exitCalled = true }
	defer func() { exitFunc = oldExit }()

	promoteBulkJobs("http://invalid-host", "", "")

	out := buf.String()
	assert.Contains(t, out, "Failed to connect to orchestrator")
	assert.True(t, exitCalled)
}

func TestPromoteBulkJobs_DecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`invalid json`))
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

	promoteBulkJobs(server.URL, "m", "t")

	out := buf.String()
	assert.Contains(t, out, "Failed to decode response")
	assert.True(t, exitCalled)
}

func TestPromoteBulkJobs_InvalidURL(t *testing.T) {
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	exitCalled := false
	oldExit := exitFunc
	exitFunc = func(code int) { exitCalled = true }
	defer func() { exitFunc = oldExit }()

	promoteBulkJobs("::invalid_host", "m", "t")

	out := buf.String()
	assert.Contains(t, out, "Failed to parse URL")

	assert.True(t, exitCalled)
}
