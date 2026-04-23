package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestUpdatePriority(t *testing.T) {
	// Setup mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "/jobs/TEST-123/priority", r.URL.Path)

		var body bytes.Buffer
		body.ReadFrom(r.Body)
		assert.Contains(t, body.String(), `"priority": 10`)

		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"priority": 10}`)
	}))
	defer server.Close()

	// Capture output
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitFunc = func(code int) {}
	defer func() { exitFunc = oldExit }()

	// Reset viper
	viper.Reset()
	defer viper.Reset()

	// Call function
	updatePriority(server.URL, "TEST-123", 10)

	// Verify output
	output := buf.String()
	assert.Contains(t, output, "Job TEST-123 priority updated to 10")
}

func TestUpdatePriority_Error(t *testing.T) {
	// Setup mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintln(w, "job TEST-123 not found")
	}))
	defer server.Close()

	// Capture output
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	exitCalled := false
	oldExit := exitFunc
	exitFunc = func(code int) { exitCalled = true }
	defer func() { exitFunc = oldExit }()

	// Reset viper
	viper.Reset()
	defer viper.Reset()

	// Call function
	updatePriority(server.URL, "TEST-123", 10)

	// Verify output
	output := buf.String()
	assert.Contains(t, output, "Failed to update priority")
	assert.Contains(t, output, "job TEST-123 not found")
	assert.True(t, exitCalled)
}

func TestUpdateBulkPriority_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/priority", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)

		tag := r.URL.Query().Get("tag")
		assert.Equal(t, "backend", tag)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"updated": 3}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	updateBulkPriority(server.URL, "", "backend", 10)

	assert.Contains(t, buf.String(), "Successfully updated priority for 3 jobs.")
}

func TestUpdatePriority_ConnectionError(t *testing.T) {
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	updatePriority("http://localhost:0", "TEST-123", 5)

	assert.Contains(t, buf.String(), "Failed to connect to orchestrator")
	assert.Equal(t, 1, exitCode)
}

func TestUpdateBulkPriority_Error(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/priority", func(w http.ResponseWriter, r *http.Request) {
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

	updateBulkPriority(server.URL, "match-me", "", 10)

	assert.Contains(t, buf.String(), "Failed to update bulk priority: internal server error")
	assert.Equal(t, 1, exitCode)
}

func TestUpdateBulkPriority_ConnectionError(t *testing.T) {
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	updateBulkPriority("http://localhost:0", "match-me", "", 10)

	assert.Contains(t, buf.String(), "Failed to connect to orchestrator")
	assert.Equal(t, 1, exitCode)
}

func TestUpdateBulkPriority_DecodeError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/priority", func(w http.ResponseWriter, r *http.Request) {
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

	updateBulkPriority(server.URL, "", "", 10)

	assert.Contains(t, buf.String(), "Failed to decode response")
	assert.Equal(t, 1, exitCode)
}
