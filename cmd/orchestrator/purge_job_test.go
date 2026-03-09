package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPurgeJob(t *testing.T) {
	originalExit := exitFunc
	defer func() { exitFunc = originalExit }()

	var exitCode int
	exitFunc = func(code int) {
		exitCode = code
	}

	originalStdout := stdout
	var buf bytes.Buffer
	stdout = &buf
	defer func() { stdout = originalStdout }()

	t.Run("Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/history/job-1", r.URL.Path)
			assert.Equal(t, http.MethodDelete, r.Method)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		purgeJob(server.URL, "job-1")
		assert.Equal(t, 0, exitCode)
		assert.Contains(t, buf.String(), "Job job-1 purged successfully")
	})

	t.Run("ConnectionError", func(t *testing.T) {
		exitCode = 0
		buf.Reset()
		purgeJob("http://invalid-host", "job-1")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to connect to orchestrator")
	})

	t.Run("APIError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintln(w, "job not found")
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		purgeJob(server.URL, "job-1")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to purge job: job not found")
	})

	t.Run("InvalidURL", func(t *testing.T) {
		exitCode = 0
		buf.Reset()
		purgeJob("::invalid-url", "job-1")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to create request")
	})
}
