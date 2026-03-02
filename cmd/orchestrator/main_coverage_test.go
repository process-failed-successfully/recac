package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"recac/internal/orchestrator"

	"github.com/stretchr/testify/assert"
)

func TestListJobs(t *testing.T) {
	// Mock exitFunc and stdout
	originalExit := exitFunc
	originalStdout := stdout
	var buf bytes.Buffer
	stdout = &buf
	var exitCode int
	exitFunc = func(code int) {
		exitCode = code
	}
	defer func() {
		exitFunc = originalExit
		stdout = originalStdout
	}()

	t.Run("Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/jobs", r.URL.Path)
			jobs := []orchestrator.JobInfo{
				{
					ID:        "job-1",
					Summary:   "Test Job",
					Status:    "Running",
					StartTime: time.Now(),
				},
			}
			json.NewEncoder(w).Encode(jobs)
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		listJobs(server.URL, false)

		assert.Equal(t, 0, exitCode)
		assert.Contains(t, buf.String(), "job-1")
		assert.Contains(t, buf.String(), "Test Job")
		assert.Contains(t, buf.String(), "Running")
	})

	t.Run("History", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "all", r.URL.Query().Get("state"))
			json.NewEncoder(w).Encode([]orchestrator.JobInfo{})
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		listJobs(server.URL, true)

		assert.Equal(t, 0, exitCode)
		assert.Contains(t, buf.String(), "No active jobs")
	})

	t.Run("ConnectionError", func(t *testing.T) {
		exitCode = 0
		buf.Reset()
		listJobs("http://invalid-host", false)
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to connect")
	})

	t.Run("ServerError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Error", http.StatusInternalServerError)
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		listJobs(server.URL, false)
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to fetch jobs")
	})

	t.Run("DecodeError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("invalid json"))
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		listJobs(server.URL, false)
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to decode response")
	})
}

func TestGetLogs(t *testing.T) {
	// Mock exitFunc and stdout
	originalExit := exitFunc
	originalStdout := stdout
	var buf bytes.Buffer
	stdout = &buf
	var exitCode int
	exitFunc = func(code int) {
		exitCode = code
	}
	defer func() {
		exitFunc = originalExit
		stdout = originalStdout
	}()

	t.Run("Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/jobs/job-1/logs", r.URL.Path)
			w.Write([]byte("log line 1\nlog line 2"))
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		getLogs(server.URL, "job-1")

		assert.Equal(t, 0, exitCode)
		assert.Contains(t, buf.String(), "log line 1")
		assert.Contains(t, buf.String(), "log line 2")
	})

	t.Run("NotFound", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		getLogs(server.URL, "job-unknown")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to fetch logs")
	})

	t.Run("ConnectionError", func(t *testing.T) {
		exitCode = 0
		buf.Reset()
		getLogs("http://invalid-host", "job-1")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to connect")
	})
}

func TestInspectJob(t *testing.T) {
	// Mock exitFunc and stdout
	originalExit := exitFunc
	originalStdout := stdout
	var buf bytes.Buffer
	stdout = &buf
	var exitCode int
	exitFunc = func(code int) {
		exitCode = code
	}
	defer func() {
		exitFunc = originalExit
		stdout = originalStdout
	}()

	t.Run("Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/jobs/job-1", r.URL.Path)
			job := orchestrator.JobInfo{
				ID:        "job-1",
				Summary:   "Test Job",
				Status:    "Running",
				StartTime: time.Now(),
				WorkItem: orchestrator.WorkItem{
					ID:          "job-1",
					RepoURL:     "http://repo",
					Description: "Desc",
					EnvVars:     map[string]string{"SECRET_TOKEN": "123"},
				},
			}
			json.NewEncoder(w).Encode(job)
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		inspectJob(server.URL, "job-1")

		assert.Equal(t, 0, exitCode)
		assert.Contains(t, buf.String(), "Job Details: job-1")
		assert.Contains(t, buf.String(), "***") // Secret masked
	})

	t.Run("ConnectionError", func(t *testing.T) {
		exitCode = 0
		buf.Reset()
		inspectJob("http://invalid-host", "job-1")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to connect")
	})

	t.Run("NotFound", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		inspectJob(server.URL, "job-1")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to fetch job details")
	})

	t.Run("DecodeError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("invalid json"))
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		inspectJob(server.URL, "job-1")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to decode response")
	})
}

func TestCancelJob(t *testing.T) {
	// Mock exitFunc and stdout
	originalExit := exitFunc
	originalStdout := stdout
	var buf bytes.Buffer
	stdout = &buf
	var exitCode int
	exitFunc = func(code int) {
		exitCode = code
	}
	defer func() {
		exitFunc = originalExit
		stdout = originalStdout
	}()

	t.Run("Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodDelete, r.Method)
			assert.Equal(t, "/jobs/job-1", r.URL.Path)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		cancelJob(server.URL, "job-1")

		assert.Equal(t, 0, exitCode)
		assert.Contains(t, buf.String(), "cancelled successfully")
	})

	t.Run("Failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Not Found", http.StatusNotFound)
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		cancelJob(server.URL, "job-1")

		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to cancel job")
	})

	t.Run("ConnectionError", func(t *testing.T) {
		exitCode = 0
		buf.Reset()
		cancelJob("http://invalid-host", "job-1")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to connect")
	})
}

func TestRetryJob(t *testing.T) {
	// Mock exitFunc and stdout
	originalExit := exitFunc
	originalStdout := stdout
	var buf bytes.Buffer
	stdout = &buf
	var exitCode int
	exitFunc = func(code int) {
		exitCode = code
	}
	defer func() {
		exitFunc = originalExit
		stdout = originalStdout
	}()

	t.Run("Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "/jobs/job-1/retry", r.URL.Path)
			w.WriteHeader(http.StatusAccepted)
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		retryJob(server.URL, "job-1")

		assert.Equal(t, 0, exitCode)
		assert.Contains(t, buf.String(), "retry submitted")
	})

	t.Run("Failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Conflict", http.StatusConflict)
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		retryJob(server.URL, "job-1")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to retry job")
	})

	t.Run("ConnectionError", func(t *testing.T) {
		exitCode = 0
		buf.Reset()
		retryJob("http://invalid-host", "job-1")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to connect")
	})
}

func TestRetryFailedJobs(t *testing.T) {
	// Mock exitFunc and stdout
	originalExit := exitFunc
	originalStdout := stdout
	var buf bytes.Buffer
	stdout = &buf
	var exitCode int
	exitFunc = func(code int) {
		exitCode = code
	}
	defer func() {
		exitFunc = originalExit
		stdout = originalStdout
	}()

	t.Run("Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "/jobs/retry-failed", r.URL.Path)
			fmt.Fprint(w, `{"retried": 5}`)
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		retryFailedJobs(server.URL, "")

		assert.Equal(t, 0, exitCode)
		assert.Contains(t, buf.String(), "Successfully retried 5 failed jobs")
	})

	t.Run("Failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Error", http.StatusInternalServerError)
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		retryFailedJobs(server.URL, "")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to retry failed jobs")
	})

	t.Run("DecodeError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("invalid json"))
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		retryFailedJobs(server.URL, "")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to decode response")
	})

	t.Run("ConnectionError", func(t *testing.T) {
		exitCode = 0
		buf.Reset()
		retryFailedJobs("http://localhost:0", "")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to connect")
	})
}

func TestPauseResumeOrchestrator(t *testing.T) {
	// Mock exitFunc and stdout
	originalExit := exitFunc
	originalStdout := stdout
	var buf bytes.Buffer
	stdout = &buf
	var exitCode int
	exitFunc = func(code int) {
		exitCode = code
	}
	defer func() {
		exitFunc = originalExit
		stdout = originalStdout
	}()

	t.Run("Pause", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "/pause", r.URL.Path)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		pauseOrchestrator(server.URL)

		assert.Equal(t, 0, exitCode)
		assert.Contains(t, buf.String(), "paused")
	})

	t.Run("Resume", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "/resume", r.URL.Path)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		resumeOrchestrator(server.URL)

		assert.Equal(t, 0, exitCode)
		assert.Contains(t, buf.String(), "resumed")
	})

	t.Run("PauseError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Error", http.StatusInternalServerError)
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		pauseOrchestrator(server.URL)
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to pause orchestrator")
	})

	t.Run("ResumeError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Error", http.StatusInternalServerError)
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		resumeOrchestrator(server.URL)
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to resume orchestrator")
	})

	t.Run("PauseConnectionError", func(t *testing.T) {
		exitCode = 0
		buf.Reset()
		pauseOrchestrator("http://invalid-host")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to connect")
	})

	t.Run("ResumeConnectionError", func(t *testing.T) {
		exitCode = 0
		buf.Reset()
		resumeOrchestrator("http://invalid-host")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to connect")
	})
}
