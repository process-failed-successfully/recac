package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestListTags(t *testing.T) {
	// Start a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/tags", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[
			{"name": "backend", "count": 2},
			{"name": "frontend", "count": 1}
		]`))
	}))
	defer server.Close()

	// Redirect stdout to capture output
	oldStdout := stdout
	var buf bytes.Buffer
	stdout = &buf
	defer func() { stdout = oldStdout }()

	// Mock exitFunc
	oldExitFunc := exitFunc
	exitCalled := false
	exitFunc = func(code int) {
		exitCalled = true
	}
	defer func() { exitFunc = oldExitFunc }()

	// Call the function
	listTags(server.URL)

	assert.False(t, exitCalled, "exitFunc should not be called")

	// Verify output
	output := buf.String()
	assert.Contains(t, output, "Job Tags (2)")
	assert.Contains(t, output, "Tag Name")
	assert.Contains(t, output, "Count")
	assert.Contains(t, output, "backend")
	assert.Contains(t, output, "2")
	assert.Contains(t, output, "frontend")
	assert.Contains(t, output, "1")
}

func TestListTags_Empty(t *testing.T) {
	// Start a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer server.Close()

	// Redirect stdout to capture output
	oldStdout := stdout
	var buf bytes.Buffer
	stdout = &buf
	defer func() { stdout = oldStdout }()

	oldExitFunc := exitFunc
	exitCalled := false
	exitFunc = func(code int) {
		exitCalled = true
	}
	defer func() { exitFunc = oldExitFunc }()

	listTags(server.URL)

	assert.False(t, exitCalled)
	assert.Contains(t, buf.String(), "No tags found across any jobs.")
}

func TestListTags_ErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`Internal Server Error`))
	}))
	defer server.Close()

	oldStdout := stdout
	var buf bytes.Buffer
	stdout = &buf
	defer func() { stdout = oldStdout }()

	oldExitFunc := exitFunc
	exitCalled := false
	exitFunc = func(code int) {
		exitCalled = true
	}
	defer func() { exitFunc = oldExitFunc }()

	listTags(server.URL)

	assert.True(t, exitCalled)
	assert.Contains(t, buf.String(), "Failed to fetch tags: status 500")
}

func TestListTags_DecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{invalid json`))
	}))
	defer server.Close()

	oldStdout := stdout
	var buf bytes.Buffer
	stdout = &buf
	defer func() { stdout = oldStdout }()

	oldExitFunc := exitFunc
	exitCalled := false
	exitFunc = func(code int) {
		exitCalled = true
	}
	defer func() { exitFunc = oldExitFunc }()

	listTags(server.URL)

	assert.True(t, exitCalled)
	assert.Contains(t, buf.String(), "Failed to decode response")
}

func TestListTags_ConnectionError(t *testing.T) {
	oldStdout := stdout
	var buf bytes.Buffer
	stdout = &buf
	defer func() { stdout = oldStdout }()

	oldExitFunc := exitFunc
	exitCalled := false
	exitFunc = func(code int) {
		exitCalled = true
	}
	defer func() { exitFunc = oldExitFunc }()

	// Connect to an invalid host
	listTags(fmt.Sprintf("http://localhost:%d", 0))

	assert.True(t, exitCalled)
	assert.Contains(t, buf.String(), "Failed to connect to orchestrator")
}