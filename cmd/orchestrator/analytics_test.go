package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrintAnalytics(t *testing.T) {
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
			assert.Equal(t, "/analytics", r.URL.Path)
			assert.Equal(t, http.MethodGet, r.Method)
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, `{"total_jobs": 10, "successful_jobs": 8}`)
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		printAnalytics(server.URL, "text")
		assert.Equal(t, 0, exitCode)
		assert.Contains(t, buf.String(), "Orchestrator Analytics")
	})

	t.Run("FormatJSON", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/analytics", r.URL.Path)
			assert.Equal(t, http.MethodGet, r.Method)
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, `{"total_jobs": 15, "successful_jobs": 12}`)
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		printAnalytics(server.URL, "json")
		assert.Equal(t, 0, exitCode)

		output := buf.String()
		assert.Contains(t, output, `"total_jobs": 15`)
		assert.Contains(t, output, `"successful_jobs": 12`)
		assert.NotContains(t, output, "Orchestrator Analytics") // No human readable header
	})

	t.Run("ConnectionError", func(t *testing.T) {
		exitCode = 0
		buf.Reset()
		printAnalytics("http://invalid-host", "text")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to connect to orchestrator")
	})

	t.Run("APIError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, "internal error")
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		printAnalytics(server.URL, "text")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to fetch analytics: status 500")
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, `invalid json`)
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		printAnalytics(server.URL, "text")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to decode response")
	})
}
