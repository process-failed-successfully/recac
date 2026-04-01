package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"recac/internal/orchestrator"
)

func TestApplyPipeline_Create(t *testing.T) {
	pipelineYAML := `
name: "Test Pipeline"
jobs:
  job1:
    summary: "Test Job"
    task: "Do something"
`
	tmpFile, err := os.CreateTemp("", "test-pipeline-*.yaml")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(pipelineYAML)
	assert.NoError(t, err)
	tmpFile.Close()

	var postCalled bool

	mux := http.NewServeMux()
	mux.HandleFunc("/jobs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			// Mock active and pending jobs as empty
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("[]"))
			return
		} else if r.Method == http.MethodPost {
			postCalled = true
			var item orchestrator.WorkItem
			err := json.NewDecoder(r.Body).Decode(&item)
			assert.NoError(t, err)
			assert.Equal(t, "test-pipeline-job1", item.ID) // stable ID
			assert.Equal(t, "Test Job", item.Summary)
			w.WriteHeader(http.StatusAccepted)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	r, w, _ := os.Pipe()
	oldStdout := stdout
	stdout = w
	defer func() { stdout = oldStdout }()

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	applyPipelineJob(server.URL, tmpFile.Name(), false, "", nil, "stable")

	w.Close()
	buf := new(strings.Builder)
	io.Copy(buf, r)
	out := buf.String()

	assert.True(t, postCalled, "POST /jobs should be called")
	assert.Contains(t, out, "[CREATE] Job test-pipeline-job1")
	assert.Equal(t, 0, exitCode)
}

func TestApplyPipeline_Update(t *testing.T) {
	pipelineYAML := `
name: "Test Pipeline"
jobs:
  job1:
    summary: "Test Job Updated"
    task: "Do something"
`
	tmpFile, err := os.CreateTemp("", "test-pipeline-*.yaml")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(pipelineYAML)
	assert.NoError(t, err)
	tmpFile.Close()

	var putCalled bool

	mux := http.NewServeMux()
	mux.HandleFunc("/jobs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			state := r.URL.Query().Get("state")
			if state == "active" {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("[]"))
				return
			} else if state == "pending" {
				pendingJob := orchestrator.JobInfo{
					ID: "test-pipeline-job1",
					WorkItem: orchestrator.WorkItem{
						ID:          "test-pipeline-job1",
						Summary:     "Test Job Old", // This differs from pipeline YAML
						Description: "Do something",
					},
				}
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode([]orchestrator.JobInfo{pendingJob})
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	})

	mux.HandleFunc("/jobs/test-pipeline-job1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			putCalled = true
			var item orchestrator.WorkItem
			err := json.NewDecoder(r.Body).Decode(&item)
			assert.NoError(t, err)
			assert.Equal(t, "Test Job Updated", item.Summary)
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	r, w, _ := os.Pipe()
	oldStdout := stdout
	stdout = w
	defer func() { stdout = oldStdout }()

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	applyPipelineJob(server.URL, tmpFile.Name(), false, "", nil, "stable")

	w.Close()
	buf := new(strings.Builder)
	io.Copy(buf, r)
	out := buf.String()

	assert.True(t, putCalled, "PUT /jobs/test-pipeline-job1 should be called")
	assert.Contains(t, out, "[UPDATE] Job test-pipeline-job1")
	assert.Equal(t, 0, exitCode)
}

func TestApplyPipeline_SkipActive(t *testing.T) {
	pipelineYAML := `
name: "Test Pipeline"
jobs:
  job1:
    summary: "Test Job Updated"
    task: "Do something"
`
	tmpFile, err := os.CreateTemp("", "test-pipeline-*.yaml")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(pipelineYAML)
	assert.NoError(t, err)
	tmpFile.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/jobs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			state := r.URL.Query().Get("state")
			if state == "active" {
				activeJob := orchestrator.JobInfo{
					ID: "test-pipeline-job1",
				}
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode([]orchestrator.JobInfo{activeJob})
				return
			} else if state == "pending" {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("[]"))
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	r, w, _ := os.Pipe()
	oldStdout := stdout
	stdout = w
	defer func() { stdout = oldStdout }()

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	applyPipelineJob(server.URL, tmpFile.Name(), false, "", nil, "stable")

	w.Close()
	buf := new(strings.Builder)
	io.Copy(buf, r)
	out := buf.String()

	assert.Contains(t, out, "[SKIP] Job test-pipeline-job1 is currently active and cannot be updated")
	assert.Equal(t, 0, exitCode)
}

func TestApplyPipeline_Unchanged(t *testing.T) {
	pipelineYAML := `
name: "Test Pipeline"
jobs:
  job1:
    summary: "Test Job"
    task: "Do something"
`
	tmpFile, err := os.CreateTemp("", "test-pipeline-*.yaml")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(pipelineYAML)
	assert.NoError(t, err)
	tmpFile.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/jobs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			state := r.URL.Query().Get("state")
			if state == "active" {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("[]"))
				return
			} else if state == "pending" {
				pendingJob := orchestrator.JobInfo{
					ID: "test-pipeline-job1",
					WorkItem: orchestrator.WorkItem{
						ID:          "test-pipeline-job1",
						Summary:     "Test Job",
						Description: "Do something",
					},
				}
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode([]orchestrator.JobInfo{pendingJob})
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	})

	// If PUT is called, the test should fail as it should be recognized as unchanged

	server := httptest.NewServer(mux)
	defer server.Close()

	r, w, _ := os.Pipe()
	oldStdout := stdout
	stdout = w
	defer func() { stdout = oldStdout }()

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	applyPipelineJob(server.URL, tmpFile.Name(), false, "", nil, "stable")

	w.Close()
	buf := new(strings.Builder)
	io.Copy(buf, r)
	out := buf.String()

	assert.Contains(t, out, "[UNCHANGED] Job test-pipeline-job1")
	assert.Equal(t, 0, exitCode)
}

func TestApplyPipelineErrors(t *testing.T) {
	// Set up a mock server that returns errors
	errorMux := http.NewServeMux()
	errorMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, "Internal Server Error")
	})
	errorServer := httptest.NewServer(errorMux)
	defer errorServer.Close()

	// Redirect stdout to buffer
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	// Dummy exit func
	oldExit := exitFunc
	exitFunc = func(code int) {
		if code != 0 {
			panic(fmt.Sprintf("exit %d", code))
		}
	}
	defer func() { exitFunc = oldExit }()

	t.Run("InvalidFilePath", func(t *testing.T) {
		buf.Reset()
		err := applyPipelineJobInternal(errorServer.URL, "nonexistent.yaml", false, "", nil, "stable")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Failed to read file nonexistent.yaml")
	})

	t.Run("InvalidPipelineContent", func(t *testing.T) {
		buf.Reset()
		tmpFile, err := os.CreateTemp("", "invalid_pipeline_*.yaml")
		assert.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		tmpFile.WriteString("invalid: yaml: content\n  - :")
		tmpFile.Close()

		err = applyPipelineJobInternal(errorServer.URL, tmpFile.Name(), false, "", nil, "stable")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Pipeline validation failed")
	})

	t.Run("FetchActiveJobsError", func(t *testing.T) {
		buf.Reset()
		tmpFile, err := os.CreateTemp("", "pipeline_active_err_*.yaml")
		assert.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		validPipeline := `name: test
jobs:
  job1:
    image: alpine`
		tmpFile.WriteString(validPipeline)
		tmpFile.Close()

		err = applyPipelineJobInternal("http://localhost:0", tmpFile.Name(), false, "", nil, "stable")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Failed to fetch active jobs")
	})

	t.Run("FetchPendingJobsError", func(t *testing.T) {
		buf.Reset()
		tmpFile, err := os.CreateTemp("", "pipeline_pending_err_*.yaml")
		assert.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		validPipeline := `name: test
jobs:
  job1:
    image: alpine`
		tmpFile.WriteString(validPipeline)
		tmpFile.Close()

		// Mock server that succeeds on active, fails on pending
		mux := http.NewServeMux()
		mux.HandleFunc("/jobs", func(w http.ResponseWriter, r *http.Request) {
			state := r.URL.Query().Get("state")
			if state == "active" {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("[]"))
			} else {
				// force error
				conn, _, _ := w.(http.Hijacker).Hijack()
				conn.Close()
			}
		})
		customServer := httptest.NewServer(mux)
		defer customServer.Close()

		err = applyPipelineJobInternal(customServer.URL, tmpFile.Name(), false, "", nil, "stable")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Failed to fetch pending jobs")
	})

	t.Run("ApplyPipelineJobWrapperPanic", func(t *testing.T) {
		buf.Reset()
		assert.PanicsWithValue(t, "exit 1", func() {
			applyPipelineJob(errorServer.URL, "nonexistent.yaml", false, "", nil, "stable")
		})
		assert.Contains(t, buf.String(), "Failed to read file nonexistent.yaml")
	})
}

func TestApplyPipelineErrors_UpdateCreate(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "pipeline_update_create_err_*.yaml")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	validPipeline := `name: test
jobs:
  job1:
    image: alpine
    command: "echo test"`
	tmpFile.WriteString(validPipeline)
	tmpFile.Close()

	// Redirect stdout to buffer
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	t.Run("UpdateError", func(t *testing.T) {
		buf.Reset()
		mux := http.NewServeMux()
		mux.HandleFunc("/jobs", func(w http.ResponseWriter, r *http.Request) {
			state := r.URL.Query().Get("state")
			if state == "active" {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("[]"))
			} else if state == "pending" {
				w.WriteHeader(http.StatusOK)
				// Return a pending job with different summary so it triggers update
				j := []orchestrator.JobInfo{{ID: "test-job1", WorkItem: orchestrator.WorkItem{ID: "test-job1", Summary: "old"}}}
				json.NewEncoder(w).Encode(j)
			}
		})
		mux.HandleFunc("/jobs/test-job1", func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPut {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("update failed"))
			}
		})
		customServer := httptest.NewServer(mux)
		defer customServer.Close()

		err := applyPipelineJobInternal(customServer.URL, tmpFile.Name(), false, "", nil, "stable")
		if assert.Error(t, err) {
			assert.Contains(t, err.Error(), "Applied pipeline with 1 errors")
		}
		assert.Contains(t, buf.String(), "Error updating job test-job1")
	})

	t.Run("CreateError", func(t *testing.T) {
		buf.Reset()
		mux := http.NewServeMux()
		mux.HandleFunc("/jobs", func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("[]")) // No active, no pending
			} else if r.Method == http.MethodPost {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("create failed"))
			}
		})
		customServer := httptest.NewServer(mux)
		defer customServer.Close()

		err := applyPipelineJobInternal(customServer.URL, tmpFile.Name(), false, "", nil, "stable")
		if assert.Error(t, err) {
			assert.Contains(t, err.Error(), "Applied pipeline with 1 errors")
		}
		assert.Contains(t, buf.String(), "Error creating job test-job1")
	})
}
