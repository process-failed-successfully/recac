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

func TestUpdateTimeout(t *testing.T) {
	// Setup mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "/jobs/TEST-123/timeout", r.URL.Path)

		var body bytes.Buffer
		body.ReadFrom(r.Body)
		assert.Contains(t, body.String(), `"timeout": "1h30m"`)

		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"timeout": "1h30m0s"}`)
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
	updateTimeout(server.URL, "TEST-123", "1h30m")

	// Verify output
	output := buf.String()
	assert.Contains(t, output, "Job TEST-123 timeout updated to 1h30m")
}

func TestUpdateTimeout_Error(t *testing.T) {
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
	updateTimeout(server.URL, "TEST-123", "1h30m")

	// Verify output
	output := buf.String()
	assert.Contains(t, output, "Failed to update timeout")
	assert.Contains(t, output, "job TEST-123 not found")
	assert.True(t, exitCalled)
}
