package main

import (
	"encoding/json"
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
