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

func TestPurgeJobsOlderThan(t *testing.T) {
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
			assert.Equal(t, "/history", r.URL.Path)
			assert.Equal(t, "24h", r.URL.Query().Get("older_than"))
			assert.Equal(t, http.MethodDelete, r.Method)
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, `{"cleared": 5}`)
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		purgeJobsOlderThan(server.URL, "24h")
		assert.Equal(t, 0, exitCode)
		assert.Contains(t, buf.String(), "Successfully purged 5 jobs older than '24h'.")
	})

	t.Run("ConnectionError", func(t *testing.T) {
		exitCode = 0
		buf.Reset()
		purgeJobsOlderThan("http://invalid-host", "24h")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to connect to orchestrator")
	})

	t.Run("APIError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintln(w, "invalid duration for older_than")
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		purgeJobsOlderThan(server.URL, "invalid")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to purge jobs older than invalid: invalid duration for older_than")
	})

	t.Run("JSONDecodeError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, `invalid json`)
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		purgeJobsOlderThan(server.URL, "24h")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to decode response")
	})

	t.Run("UnexpectedFormatError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, `{"some_other_key": "val"}`)
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		purgeJobsOlderThan(server.URL, "24h")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Unexpected response format")
	})

	t.Run("InvalidURL", func(t *testing.T) {
		exitCode = 0
		buf.Reset()
		purgeJobsOlderThan("::invalid-url", "24h")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to create request")
	})
}
