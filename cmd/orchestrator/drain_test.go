package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDrainOrchestrator(t *testing.T) {
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
			assert.Equal(t, "/drain", r.URL.Path)
			assert.Equal(t, http.MethodPost, r.Method)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		drainOrchestrator(server.URL)
		assert.Equal(t, 0, exitCode)
		assert.Contains(t, buf.String(), "Orchestrator is now draining")
	})

	t.Run("ConnectionError", func(t *testing.T) {
		exitCode = 0
		buf.Reset()
		drainOrchestrator("http://invalid-host")
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
		drainOrchestrator(server.URL)
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to set orchestrator to drain mode")
	})

	t.Run("InvalidURL", func(t *testing.T) {
		exitCode = 0
		buf.Reset()
		drainOrchestrator("::invalid-url")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to create request")
	})
}

func TestUndrainOrchestrator(t *testing.T) {
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
			assert.Equal(t, "/undrain", r.URL.Path)
			assert.Equal(t, http.MethodPost, r.Method)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		undrainOrchestrator(server.URL)
		assert.Equal(t, 0, exitCode)
		assert.Contains(t, buf.String(), "Orchestrator is no longer draining")
	})

	t.Run("ConnectionError", func(t *testing.T) {
		exitCode = 0
		buf.Reset()
		undrainOrchestrator("http://invalid-host")
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
		undrainOrchestrator(server.URL)
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to remove orchestrator from drain mode")
	})

	t.Run("InvalidURL", func(t *testing.T) {
		exitCode = 0
		buf.Reset()
		undrainOrchestrator("::invalid-url")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to create request")
	})
}
