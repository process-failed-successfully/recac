package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCancelJobsByTag(t *testing.T) {
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
			assert.Equal(t, "/jobs", r.URL.Path)
			assert.Equal(t, "my-tag", r.URL.Query().Get("tag"))
			assert.Equal(t, http.MethodDelete, r.Method)
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, `{"canceled": 3}`)
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		cancelJobsByTag(server.URL, "my-tag")
		assert.Equal(t, 0, exitCode)
		assert.Contains(t, buf.String(), "Successfully canceled 3 jobs with tag 'my-tag'.")
	})

	t.Run("ConnectionError", func(t *testing.T) {
		exitCode = 0
		buf.Reset()
		cancelJobsByTag("http://invalid-host", "my-tag")
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
		cancelJobsByTag(server.URL, "my-tag")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to cancel jobs by tag: internal error")
	})

	t.Run("InvalidURL", func(t *testing.T) {
		exitCode = 0
		buf.Reset()
		cancelJobsByTag("::invalid-url", "my-tag")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to create request")
	})
}

func TestCancelJobsByTag_InvalidResponse(t *testing.T) {
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

	t.Run("InvalidJSON", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, "invalid json")
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		cancelJobsByTag(server.URL, "my-tag")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to decode response")
	})

	t.Run("MissingCanceledField", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, `{"other": 3}`)
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		cancelJobsByTag(server.URL, "my-tag")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Unexpected response format")
	})
}
