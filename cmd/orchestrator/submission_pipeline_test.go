package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSubmitPipelineJob_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs/pipeline", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/x-yaml", r.Header.Get("Content-Type"))

		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{"submitted": ["pipeline-test-build-123", "pipeline-test-test-123"], "errors": []}`))
	}))
	defer server.Close()

	f, _ := os.CreateTemp("", "pipeline_*.yaml")
	f.Write([]byte("name: pipeline\njobs:\n  build:\n    summary: build\n"))
	f.Close()
	defer os.Remove(f.Name())

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	submitPipelineJob(server.URL, f.Name(), false)

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Pipeline submission completed.")
	assert.Contains(t, string(out), "Successfully submitted jobs: pipeline-test-build-123, pipeline-test-test-123")
}

func TestSubmitPipelineJob_InvalidFile(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	submitPipelineJob("http://localhost", "non_existent_pipeline_file.yaml", false)

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Failed to open file non_existent_pipeline_file.yaml")
	assert.Equal(t, 1, exitCode)
}

func TestSubmitPipelineJob_ConnectionError(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	f, _ := os.CreateTemp("", "pipeline_*.yaml")
	f.Write([]byte("name: pipeline\njobs:\n  build:\n    summary: build\n"))
	f.Close()
	defer os.Remove(f.Name())

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	submitPipelineJob("http://localhost:123456", f.Name(), false)

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Failed to connect to orchestrator")
	assert.Equal(t, 1, exitCode)
}

func TestSubmitPipelineJob_ErrorResponse(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	f, _ := os.CreateTemp("", "pipeline_*.yaml")
	f.Write([]byte("name: pipeline\njobs:\n  build:\n    summary: build\n"))
	f.Close()
	defer os.Remove(f.Name())

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	submitPipelineJob(server.URL, f.Name(), false)

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Failed to submit pipeline job: Internal Server Error")
	assert.Equal(t, 1, exitCode)
}

func TestSubmitPipelineJob_InvalidJSON(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	f, _ := os.CreateTemp("", "pipeline_*.yaml")
	f.Write([]byte("name: pipeline\njobs:\n  build:\n    summary: build\n"))
	f.Close()
	defer os.Remove(f.Name())

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	submitPipelineJob(server.URL, f.Name(), false)

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Failed to parse pipeline response")
	assert.Equal(t, 1, exitCode)
}

func TestSubmitPipelineJob_WithErrors(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{"submitted": ["JOB-1"], "errors": ["some error"]}`))
	}))
	defer server.Close()

	f, _ := os.CreateTemp("", "pipeline_*.yaml")
	f.Write([]byte("name: pipeline\njobs:\n  build:\n    summary: build\n"))
	f.Close()
	defer os.Remove(f.Name())

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	submitPipelineJob(server.URL, f.Name(), false)

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Successfully submitted jobs: JOB-1")
	assert.Contains(t, string(out), "Errors:")
	assert.Contains(t, string(out), "some error")
	assert.Equal(t, 0, exitCode)
}

func TestSubmitPipelineJob_WaitFailed(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusAccepted)
			w.Write([]byte(`{"submitted": ["JOB-WAIT"], "errors": []}`))
		} else if r.Method == http.MethodGet {
			callCount++
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": "Failed", "error": "test pipeline failure"}`))
		}
	}))
	defer server.Close()

	f, _ := os.CreateTemp("", "pipeline_*.yaml")
	f.Write([]byte("name: pipeline\njobs:\n  build:\n    summary: build\n"))
	f.Close()
	defer os.Remove(f.Name())

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	submitPipelineJob(server.URL, f.Name(), true)

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Job JOB-WAIT failed: job failed with error: test pipeline failure")
	assert.Equal(t, 1, exitCode)
	assert.Greater(t, callCount, 0)
}
