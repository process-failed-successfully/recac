package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"recac/internal/orchestrator"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubmitJob_Success(t *testing.T) {
	// Mock Exit and Stdout
	var exitCode int
	originalExit := exitFunc
	exitFunc = func(code int) {
		exitCode = code
	}
	defer func() { exitFunc = originalExit }()

	var out bytes.Buffer
	originalStdout := stdout
	stdout = &out
	defer func() { stdout = originalStdout }()

	// Mock Server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs", r.URL.Path)
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte("Job submitted"))
	}))
	defer server.Close()

	// Create temp file
	tmpDir, err := os.MkdirTemp("", "submit-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "job.json")
	item := orchestrator.WorkItem{ID: "JOB-1", Summary: "Test Job"}
	data, _ := json.Marshal(item)
	os.WriteFile(filePath, data, 0644)

	// Execute
	submitJob(server.URL, filePath, false)

	// Verify
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, out.String(), "Job submitted")
}

func TestSubmitJob_FileNotFound(t *testing.T) {
	var exitCode int
	originalExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = originalExit }()

	var out bytes.Buffer
	originalStdout := stdout
	stdout = &out
	defer func() { stdout = originalStdout }()

	submitJob("http://localhost", "non-existent.json", false)

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to open file")
}

func TestSubmitJob_InvalidJSON(t *testing.T) {
	var exitCode int
	originalExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = originalExit }()

	var out bytes.Buffer
	originalStdout := stdout
	stdout = &out
	defer func() { stdout = originalStdout }()

	tmpDir, _ := os.MkdirTemp("", "submit-test-invalid")
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "invalid.json")
	os.WriteFile(filePath, []byte("invalid json"), 0644)

	submitJob("http://localhost", filePath, false)

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Invalid JSON")
}

func TestSubmitAdHocJob_ServerError(t *testing.T) {
	var exitCode int
	originalExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = originalExit }()

	var out bytes.Buffer
	originalStdout := stdout
	stdout = &out
	defer func() { stdout = originalStdout }()

	// Closed server
	server := httptest.NewServer(http.HandlerFunc(nil))
	server.Close()

	submitAdHocJob(server.URL, "http://repo.com", "Task", "ID", 0, 0, 0, nil, nil, nil, nil, nil, false, nil, nil, nil, "", false, "", "", "", "", false)
	submitAdHocJob(server.URL, "http://repo.com", "Task", "ID", 0, 0, 0, nil, nil, nil, nil, nil, false, nil, nil, nil, "", false, "", "", "", "", false)

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to connect")
}

func TestSubmitAdHocJob_ErrorResponse(t *testing.T) {
	var exitCode int
	originalExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = originalExit }()

	var out bytes.Buffer
	originalStdout := stdout
	stdout = &out
	defer func() { stdout = originalStdout }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	submitAdHocJob(server.URL, "http://repo.com", "Task", "ID", 0, 0, 0, nil, nil, nil, nil, nil, false, nil, nil, nil, "", false, "", "", "", "", false)

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to submit job")
	assert.Contains(t, out.String(), "Internal Server Error")
}

func TestWaitForJob_Completed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs/JOB-1" {
			json.NewEncoder(w).Encode(orchestrator.JobInfo{Status: "Completed"})
		}
	}))
	defer server.Close()

	var out bytes.Buffer
	err := waitForJob(server.URL, "JOB-1", &out)

	assert.NoError(t, err)
	assert.Contains(t, out.String(), "Job already completed")
}

func TestSubmitJob_WithWait(t *testing.T) {
	// Mock Exit and Stdout
	var exitCode int
	originalExit := exitFunc
	exitFunc = func(code int) {
		exitCode = code
	}
	defer func() { exitFunc = originalExit }()

	var out bytes.Buffer
	originalStdout := stdout
	stdout = &out
	defer func() { stdout = originalStdout }()

	// Mock Server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusAccepted)
			w.Write([]byte("Job submitted"))
			return
		}
		if r.URL.Path == "/jobs/JOB-WAIT" {
			// Return completed immediately for simplicity
			json.NewEncoder(w).Encode(orchestrator.JobInfo{Status: "Completed"})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	// Create temp file
	tmpDir, err := os.MkdirTemp("", "submit-test-wait")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "job.json")
	// Use manual JSON to ensure lowercase "id" key as expected by submitJob
	data := []byte(`{"id": "JOB-WAIT", "summary": "Test Job"}`)
	os.WriteFile(filePath, data, 0644)

	// Execute
	submitJob(server.URL, filePath, true)

	// Verify
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, out.String(), "Job submitted")
	assert.Contains(t, out.String(), "Job already completed")
}

func TestSubmitBatchJob(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/batch", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{"submitted":["BATCH-1", "BATCH-2"],"errors":[]}`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	file, err := os.CreateTemp("", "batch-job-*.json")
	assert.NoError(t, err)
	defer os.Remove(file.Name())

	_, err = file.WriteString(`[{"id":"BATCH-1","summary":"Test"},{"id":"BATCH-2","summary":"Test"}]`)
	assert.NoError(t, err)
	file.Close()

	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	submitBatchJob(server.URL, file.Name(), false)

	assert.Equal(t, 0, exitCode)
	assert.Contains(t, out.String(), "Batch submission completed.")
	assert.Contains(t, out.String(), "BATCH-1")
	assert.Contains(t, out.String(), "BATCH-2")
}

func TestSubmitBatchJob_PartialFailure(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/batch", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{"submitted":["BATCH-1"],"errors":["BATCH-2: already active"]}`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	file, err := os.CreateTemp("", "batch-job-*.json")
	assert.NoError(t, err)
	defer os.Remove(file.Name())

	_, err = file.WriteString(`[{"id":"BATCH-1","summary":"Test"},{"id":"BATCH-2","summary":"Test"}]`)
	assert.NoError(t, err)
	file.Close()

	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	submitBatchJob(server.URL, file.Name(), false)

	assert.Equal(t, 0, exitCode) // still exits 0 if at least 1 job submitted
	assert.Contains(t, out.String(), "Batch submission completed.")
	assert.Contains(t, out.String(), "BATCH-1")
	assert.Contains(t, out.String(), "BATCH-2: already active")
}

func TestSubmitBatchJob_Wait(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/batch", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{"submitted":["BATCH-1"],"errors":[]}`))
	})

	mux.HandleFunc("/jobs/BATCH-1", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"BATCH-1","status":"Completed"}`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	file, err := os.CreateTemp("", "batch-job-*.json")
	assert.NoError(t, err)
	defer os.Remove(file.Name())

	_, err = file.WriteString(`[{"id":"BATCH-1","summary":"Test"}]`)
	assert.NoError(t, err)
	file.Close()

	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	submitBatchJob(server.URL, file.Name(), true)

	assert.Equal(t, 0, exitCode)
	assert.Contains(t, out.String(), "Batch submission completed.")
	assert.Contains(t, out.String(), "Waiting for 1 jobs to complete")
	assert.Contains(t, out.String(), "All 1 jobs completed successfully")
}

func TestSubmitBatchJob_AllFailed(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/batch", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{"submitted":[],"errors":["BATCH-1: invalid"]}`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	file, err := os.CreateTemp("", "batch-job-*.json")
	assert.NoError(t, err)
	defer os.Remove(file.Name())

	_, err = file.WriteString(`[{"id":"BATCH-1","summary":"Test"}]`)
	assert.NoError(t, err)
	file.Close()

	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	submitBatchJob(server.URL, file.Name(), false)

	assert.Equal(t, 1, exitCode) // all failed should exit 1
	assert.Contains(t, out.String(), "BATCH-1: invalid")
}

func TestSubmitBatchJob_InvalidJSON(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/batch", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`Invalid JSON array body`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	file, err := os.CreateTemp("", "batch-job-*.json")
	assert.NoError(t, err)
	defer os.Remove(file.Name())

	_, err = file.WriteString(`{"id":"BATCH-1","summary":"Test"}`) // not an array
	assert.NoError(t, err)
	file.Close()

	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	submitBatchJob(server.URL, file.Name(), false)

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to submit batch job:")
}

func TestSetJobOutput_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/JOB-123/output", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)

		var reqBody struct {
			Outputs map[string]string `json:"outputs"`
		}
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		assert.NoError(t, err)
		assert.Equal(t, "val1", reqBody.Outputs["key1"])

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "success"}`))
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

	setJobOutput(server.URL, "JOB-123", "key1", "val1")

	w.Close()
	buf := new(strings.Builder)
	_, _ = io.Copy(buf, r)
	out := buf.String()

	assert.Contains(t, out, "Successfully set output key1=val1 for job JOB-123")
	assert.Equal(t, 0, exitCode)
}

func TestAddJobMetrics_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/JOB-123/metrics", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)

		var reqBody struct {
			Metrics map[string]float64 `json:"metrics"`
		}
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		assert.NoError(t, err)
		assert.Equal(t, 42.5, reqBody.Metrics["key1"])

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "success"}`))
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

	addJobMetrics(server.URL, "JOB-123", "key1", 42.5)

	w.Close()
	buf := new(strings.Builder)
	_, _ = io.Copy(buf, r)
	out := buf.String()

	assert.Contains(t, out, "Successfully added metric key1=42.50 for job JOB-123")
	assert.Equal(t, 0, exitCode)
}

func TestAddJobMetrics_Error(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/JOB-123/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`invalid metrics`))
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

	addJobMetrics(server.URL, "JOB-123", "key1", 42.5)

	w.Close()
	buf := new(strings.Builder)
	_, _ = io.Copy(buf, r)
	out := buf.String()

	assert.Contains(t, out, "Failed to add job metrics: invalid metrics")
	assert.Equal(t, 1, exitCode)
}

func TestSetJobOutput_Error(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/JOB-123/output", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`invalid output`))
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

	setJobOutput(server.URL, "JOB-123", "key1", "val1")

	w.Close()
	buf := new(strings.Builder)
	_, _ = io.Copy(buf, r)
	out := buf.String()

	assert.Contains(t, out, "Failed to set job output: invalid output")
	assert.Equal(t, 1, exitCode)
}

func TestSubmitAdHocJob_WithWait(t *testing.T) {
	// Mock Exit and Stdout
	var exitCode int
	originalExit := exitFunc
	exitFunc = func(code int) {
		exitCode = code
	}
	defer func() { exitFunc = originalExit }()

	var out bytes.Buffer
	originalStdout := stdout
	stdout = &out
	defer func() { stdout = originalStdout }()

	// Mock Server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusAccepted)
			w.Write([]byte("Job submitted"))
			return
		}
		if r.URL.Path == "/jobs/JOB-ADHOC-WAIT" {
			// Return completed immediately for simplicity
			json.NewEncoder(w).Encode(orchestrator.JobInfo{Status: "Completed"})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	// Execute
	submitAdHocJob(server.URL, "http://repo.com", "Task", "JOB-ADHOC-WAIT", 0, 0, 0, nil, nil, nil, nil, nil, true, nil, nil, nil, "", false, "", "", "", "", false)

	// Verify
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, out.String(), "Job submitted")
	assert.Contains(t, out.String(), "Job already completed")
}

func TestGetJobOutput_Success(t *testing.T) {
	var exitCode int
	originalExit := exitFunc
	exitFunc = func(code int) {
		exitCode = code
	}
	defer func() { exitFunc = originalExit }()

	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/JOB-123", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		job := orchestrator.JobInfo{
			ID: "JOB-123",
			Outputs: map[string]string{
				"key1": "val1",
			},
		}
		json.NewEncoder(w).Encode(job)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	var outBuf bytes.Buffer
	originalStdout := stdout
	stdout = &outBuf
	defer func() { stdout = originalStdout }()

	getJobOutput(server.URL, "JOB-123", "key1")

	assert.Contains(t, outBuf.String(), "val1\n")
	assert.Equal(t, 0, exitCode)
}

func TestGetJobOutput_NotFound(t *testing.T) {
	var exitCode int
	originalExit := exitFunc
	exitFunc = func(code int) {
		exitCode = code
	}
	defer func() { exitFunc = originalExit }()

	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/JOB-123", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Not Found"))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	var outBuf bytes.Buffer
	originalStdout := stdout
	stdout = &outBuf
	defer func() { stdout = originalStdout }()

	getJobOutput(server.URL, "JOB-123", "key1")

	assert.Contains(t, outBuf.String(), "Failed to get job info: Not Found")
	assert.Equal(t, 1, exitCode)
}
