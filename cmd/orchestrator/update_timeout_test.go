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

func TestUpdateBulkTimeout(t *testing.T) {
	// Setup mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "/jobs/timeout", r.URL.Path)

		query := r.URL.Query()
		tag := query.Get("tag")
		match := query.Get("match")

		var body bytes.Buffer
		body.ReadFrom(r.Body)

		if tag == "backend" && match == "" {
			assert.Contains(t, body.String(), `"timeout":"1h"`)
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, `{"updated": 3}`)
		} else if tag == "" && match == "Fix" {
			assert.Contains(t, body.String(), `"timeout":"45m"`)
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, `{"updated": 5}`)
		} else {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintln(w, "invalid request")
		}
	}))
	defer server.Close()

	t.Run("ByTag", func(t *testing.T) {
		var buf bytes.Buffer
		oldStdout := stdout
		stdout = &buf
		defer func() { stdout = oldStdout }()

		oldExit := exitFunc
		exitFunc = func(code int) {}
		defer func() { exitFunc = oldExit }()

		viper.Reset()

		updateBulkTimeout(server.URL, "", "backend", "1h")

		output := buf.String()
		assert.Contains(t, output, "Successfully updated timeouts for 3 pending jobs.")
	})

	t.Run("ByMatch", func(t *testing.T) {
		var buf bytes.Buffer
		oldStdout := stdout
		stdout = &buf
		defer func() { stdout = oldStdout }()

		oldExit := exitFunc
		exitFunc = func(code int) {}
		defer func() { exitFunc = oldExit }()

		viper.Reset()

		updateBulkTimeout(server.URL, "Fix", "", "45m")

		output := buf.String()
		assert.Contains(t, output, "Successfully updated timeouts for 5 pending jobs.")
	})

	t.Run("Error", func(t *testing.T) {
		var buf bytes.Buffer
		oldStdout := stdout
		stdout = &buf
		defer func() { stdout = oldStdout }()

		exitCalled := false
		oldExit := exitFunc
		exitFunc = func(code int) { exitCalled = true }
		defer func() { exitFunc = oldExit }()

		viper.Reset()

		updateBulkTimeout(server.URL, "invalid", "request", "1h")

		output := buf.String()
		assert.Contains(t, output, "Failed to update bulk timeouts")
		assert.True(t, exitCalled)
	})
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
