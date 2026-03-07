package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSubmitJob_Errors(t *testing.T) {
	// Mock exitFunc
	originalExit := exitFunc
	defer func() { exitFunc = originalExit }()

	var exitCode int
	exitFunc = func(code int) {
		exitCode = code
	}

	// Mock stdout
	originalStdout := stdout
	var buf bytes.Buffer
	stdout = &buf
	defer func() { stdout = originalStdout }()

	t.Run("FileNotFound", func(t *testing.T) {
		exitCode = 0
		buf.Reset()
		submitJob("http://localhost", "non-existent-file.json", false)
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to open file")
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		tmpFile, _ := os.CreateTemp("", "invalid.json")
		defer os.Remove(tmpFile.Name())
		tmpFile.WriteString("{invalid-json")
		tmpFile.Close()

		exitCode = 0
		buf.Reset()
		submitJob("http://localhost", tmpFile.Name(), false)
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Invalid JSON in file")
	})

	t.Run("ConnectionError", func(t *testing.T) {
		tmpFile, _ := os.CreateTemp("", "valid.json")
		defer os.Remove(tmpFile.Name())
		tmpFile.WriteString(`{"id":"job-1"}`)
		tmpFile.Close()

		exitCode = 0
		buf.Reset()
		submitJob("http://invalid-host", tmpFile.Name(), false)
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to connect to orchestrator")
	})

	t.Run("ServerError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}))
		defer server.Close()

		tmpFile, _ := os.CreateTemp("", "valid.json")
		defer os.Remove(tmpFile.Name())
		tmpFile.WriteString(`{"id":"job-1"}`)
		tmpFile.Close()

		exitCode = 0
		buf.Reset()
		submitJob(server.URL, tmpFile.Name(), false)
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to submit job")
	})
}

func TestWaitForJob_Errors(t *testing.T) {
	// Mock stdout
	originalStdout := stdout
	var buf bytes.Buffer
	stdout = &buf
	defer func() { stdout = originalStdout }()

	t.Run("JobFailed", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/jobs/job-fail" {
				w.Write([]byte(`{"status": "Failed", "error": "test error"}`))
				return
			}
			http.NotFound(w, r)
		}))
		defer server.Close()

		err := waitForJob(server.URL, "job-fail", &buf)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "test error")
	})

	t.Run("StreamingLogs", func(t *testing.T) {
		jobID := "job-stream"
		// Sequence of events:
		// 1. Get job -> Status: Running
		// 2. Get logs -> stream "some logs"
		// 3. Get job (after logs) -> Status: Completed

		var logRequestCount int

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/jobs/"+jobID {
				// If logs were requested, return Completed. Else Running.
				if logRequestCount > 0 {
					w.Write([]byte(`{"status": "Completed"}`))
				} else {
					w.Write([]byte(`{"status": "Running"}`))
				}
				return
			}
			if r.URL.Path == "/jobs/"+jobID+"/logs" {
				logRequestCount++
				w.Write([]byte("some logs"))
				return
			}
			http.NotFound(w, r)
		}))
		defer server.Close()

		buf.Reset()
		err := waitForJob(server.URL, jobID, &buf)
		assert.NoError(t, err)
		assert.Contains(t, buf.String(), "--- Log Stream Start ---")
		assert.Contains(t, buf.String(), "some logs")
		assert.Contains(t, buf.String(), "--- Log Stream End ---")
	})
}

func TestClearPending_Errors(t *testing.T) {
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

	t.Run("ConnectionError", func(t *testing.T) {
		exitCode = 0
		buf.Reset()
		clearPending("http://invalid-host")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to connect to orchestrator")
	})

	t.Run("BadStatusCode", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		clearPending(server.URL)
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to clear pending jobs")
	})

	t.Run("BadJSONFormat", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"cleared": "not-a-number"}`))
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		clearPending(server.URL)
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Unexpected response format")
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`invalid-json`))
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		clearPending(server.URL)
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to decode response")
	})
}

func TestClearHistory_Errors(t *testing.T) {
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

	t.Run("ConnectionError", func(t *testing.T) {
		exitCode = 0
		buf.Reset()
		clearHistory("http://invalid-host")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to connect to orchestrator")
	})

	t.Run("BadStatusCode", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		clearHistory(server.URL)
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to clear history")
	})

	t.Run("BadJSONFormat", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"cleared": "not-a-number"}`))
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		clearHistory(server.URL)
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Unexpected response format")
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`invalid-json`))
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		clearHistory(server.URL)
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to decode response")
	})
}
