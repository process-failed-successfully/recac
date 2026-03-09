package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"recac/internal/orchestrator"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubmitAdHocJob(t *testing.T) {
	// 1. Setup mock server
	var receivedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var err error
		receivedBody, err = io.ReadAll(r.Body)
		require.NoError(t, err)

		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte("Job submitted"))
	}))
	defer server.Close()

	// 2. Call function
	envVars := map[string]string{
		"KEY1": "VALUE1",
		"KEY2": "VALUE2",
	}
	submitDeps := []string{"JOB-1", "JOB-2"}
	submitAdHocJob(server.URL, "http://repo.com", "My Task", "MY-ID", 0, false, envVars, submitDeps, nil)

	// 3. Verify payload
	var item orchestrator.WorkItem
	err := json.Unmarshal(receivedBody, &item)
	require.NoError(t, err)

	assert.Equal(t, "MY-ID", item.ID)
	assert.Equal(t, "http://repo.com", item.RepoURL)
	assert.Equal(t, "My Task", item.Summary)
	assert.Equal(t, "My Task", item.Description)
	assert.Equal(t, envVars, item.EnvVars)
	assert.Equal(t, submitDeps, item.DependsOn)
}

func TestSubmitAdHocJob_AutoID(t *testing.T) {
	var receivedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		receivedBody, err = io.ReadAll(r.Body)
		require.NoError(t, err)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	submitAdHocJob(server.URL, "http://repo.com", "My Task", "", 0, false, nil, nil, nil)

	var item orchestrator.WorkItem
	err := json.Unmarshal(receivedBody, &item)
	require.NoError(t, err)

	assert.NotEmpty(t, item.ID)
	assert.Equal(t, "http://repo.com", item.RepoURL)
}

func TestClearPending(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/pending", r.URL.Path)
		assert.Equal(t, http.MethodDelete, r.Method)

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"cleared": 3}`))
	}))
	defer server.Close()

	// Redirect stdout to capture the output
	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	clearPending(server.URL)

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Successfully cleared 3 jobs from pending queue.")
}

func TestClearHistory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/history", r.URL.Path)
		assert.Equal(t, http.MethodDelete, r.Method)

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"cleared": 5}`))
	}))
	defer server.Close()

	// Redirect stdout to capture the output
	oldStdout := stdout
	r, w, _ := os.Pipe()
	stdout = w
	defer func() {
		stdout = oldStdout
	}()

	clearHistory(server.URL)

	w.Close()
	out, _ := io.ReadAll(r)

	assert.Contains(t, string(out), "Successfully cleared 5 jobs from history.")
}

func TestCancelAllJobs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs", r.URL.Path)
		assert.Equal(t, http.MethodDelete, r.Method)

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"canceled": 3}`))
	}))
	defer server.Close()

	// Redirect stdout to capture the output
	oldStdout := stdout
	r, w, _ := os.Pipe()
	stdout = w
	defer func() {
		stdout = oldStdout
	}()

	cancelAllJobs(server.URL)

	w.Close()
	out, _ := io.ReadAll(r)

	assert.Contains(t, string(out), "Successfully canceled 3 jobs.")
}

func TestSubmitJob(t *testing.T) {
	// 1. Setup mock server
	var receivedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var err error
		receivedBody, err = io.ReadAll(r.Body)
		require.NoError(t, err)

		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte("Job submitted"))
	}))
	defer server.Close()

	// 2. Create temp file
	f, _ := os.CreateTemp("", "job_*.json")
	f.Write([]byte(`{"id": "TEST-JOB"}`))
	f.Close()
	defer os.Remove(f.Name())

	// Redirect stdout
	oldStdout := stdout
	r, w, _ := os.Pipe()
	stdout = w
	defer func() { stdout = oldStdout }()

	// 3. Call function
	submitJob(server.URL, f.Name(), false)

	w.Close()
	out, _ := io.ReadAll(r)

	// 4. Verify output
	assert.Contains(t, string(out), "Job submitted")

	// Verify payload
	var item orchestrator.WorkItem
	err := json.Unmarshal(receivedBody, &item)
	require.NoError(t, err)
	assert.Equal(t, "TEST-JOB", item.ID)
}

func TestSubmitJob_InvalidFile(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	oldStdout := stdout
	r, w, _ := os.Pipe()
	stdout = w
	defer func() { stdout = oldStdout }()

	submitJob("http://localhost:8080", "non_existent_file.json", false)
	w.Close()
	out, _ := io.ReadAll(r)

	assert.Contains(t, string(out), "Failed to open file non_existent_file.json")
	assert.Equal(t, 1, exitCode)
}

func TestSubmitJob_ConnectionError(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	f, _ := os.CreateTemp("", "valid_*.json")
	f.Write([]byte(`{"id": "test"}`))
	f.Close()
	defer os.Remove(f.Name())

	oldStdout := stdout
	r, w, _ := os.Pipe()
	stdout = w
	defer func() { stdout = oldStdout }()

	submitJob("http://localhost:123456", f.Name(), false)
	w.Close()
	out, _ := io.ReadAll(r)

	assert.Contains(t, string(out), "Failed to connect to orchestrator")
	assert.Equal(t, 1, exitCode)
}

func TestCancelAllJobs_ErrorResponse(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	// Redirect stdout to capture the output
	oldStdout := stdout
	r, w, _ := os.Pipe()
	stdout = w
	defer func() {
		stdout = oldStdout
	}()

	cancelAllJobs(server.URL)

	w.Close()
	out, _ := io.ReadAll(r)

	assert.Contains(t, string(out), "Failed to cancel jobs: Internal Server Error")
	assert.Equal(t, 1, exitCode)
}

func TestCancelAllJobs_InvalidJSON(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	// Redirect stdout to capture the output
	oldStdout := stdout
	r, w, _ := os.Pipe()
	stdout = w
	defer func() {
		stdout = oldStdout
	}()

	cancelAllJobs(server.URL)

	w.Close()
	out, _ := io.ReadAll(r)

	assert.Contains(t, string(out), "Failed to decode response: invalid character")
	assert.Equal(t, 1, exitCode)
}

func TestCancelAllJobs_MissingCanceled(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"other": "data"}`))
	}))
	defer server.Close()

	// Redirect stdout to capture the output
	oldStdout := stdout
	r, w, _ := os.Pipe()
	stdout = w
	defer func() {
		stdout = oldStdout
	}()

	cancelAllJobs(server.URL)

	w.Close()
	out, _ := io.ReadAll(r)

	assert.Contains(t, string(out), "Unexpected response format")
	assert.Equal(t, 1, exitCode)
}

func TestCancelAllJobs_ConnectionError(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	// Redirect stdout to capture the output
	oldStdout := stdout
	r, w, _ := os.Pipe()
	stdout = w
	defer func() {
		stdout = oldStdout
	}()

	// Use an invalid URL to trigger a connection error
	cancelAllJobs("http://localhost:123456")

	w.Close()
	out, _ := io.ReadAll(r)

	assert.Contains(t, string(out), "Failed to connect to orchestrator")
	assert.Equal(t, 1, exitCode)
}

func TestWaitForJob_AlreadyCompleted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "Completed"}`))
	}))
	defer server.Close()

	oldStdout := stdout
	r, w, _ := os.Pipe()
	stdout = w
	defer func() { stdout = oldStdout }()

	err := waitForJob(server.URL, "test-job", stdout)
	w.Close()
	out, _ := io.ReadAll(r)

	assert.NoError(t, err)
	assert.Contains(t, string(out), "Job already completed.")
}

func TestWaitForJob_FailedImmediately(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "Failed", "error": "some error"}`))
	}))
	defer server.Close()

	oldStdout := stdout
	r, w, _ := os.Pipe()
	stdout = w
	defer func() { stdout = oldStdout }()

	err := waitForJob(server.URL, "test-job", stdout)
	w.Close()
	io.ReadAll(r) // ignore

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "some error")
}

func TestWaitForJob_CanceledImmediately(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "Canceled", "error": "canceled by user"}`))
	}))
	defer server.Close()

	oldStdout := stdout
	r, w, _ := os.Pipe()
	stdout = w
	defer func() { stdout = oldStdout }()

	err := waitForJob(server.URL, "test-job", stdout)
	w.Close()
	io.ReadAll(r) // ignore

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "canceled with error: canceled by user")
}
