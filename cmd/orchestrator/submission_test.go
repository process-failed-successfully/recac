package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"sync/atomic"
	"testing"

	"recac/internal/orchestrator"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubmitMatrixInlineJob(t *testing.T) {
	// 1. Setup mock server
	var receivedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs/matrix", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var err error
		receivedBody, err = io.ReadAll(r.Body)
		require.NoError(t, err)

		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{"submitted": ["JOB-1", "JOB-2"], "errors": []}`))
	}))
	defer server.Close()

	// Redirect stdout to capture output
	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	// 2. Call function
	envVars := map[string]string{
		"BASE_ENV": "value",
	}
	matrix := map[string][]string{
		"OS": {"linux", "windows"},
		"GO": {"1.20", "1.21"},
	}

	submitMatrixInlineJob(server.URL, "http://repo.com", "My Matrix Task", "MATRIX-ID", 0, 0, 0, nil, nil, nil, nil, nil, false, envVars, nil, nil, "group-matrix", false, "", "", "", "", false, matrix)

	pw.Close()
	out, _ := io.ReadAll(pr)
	assert.Contains(t, string(out), "Inline matrix submission completed.")

	// 3. Verify payload
	var reqBody struct {
		BaseItem orchestrator.WorkItem `json:"base_item"`
		Matrix   map[string][]string   `json:"matrix"`
	}
	err := json.Unmarshal(receivedBody, &reqBody)
	require.NoError(t, err)

	assert.Equal(t, "MATRIX-ID", reqBody.BaseItem.ID)
	assert.Equal(t, "My Matrix Task", reqBody.BaseItem.Summary)
	assert.Equal(t, "group-matrix", reqBody.BaseItem.ConcurrencyGroup)
	assert.Equal(t, envVars, reqBody.BaseItem.EnvVars)
	assert.Equal(t, matrix, reqBody.Matrix)
}

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
	submitAdHocJob(server.URL, "http://repo.com", "My Task", "MY-ID", 0, 0, 0, nil, nil, nil, nil, nil, false, envVars, submitDeps, nil, "group-1", true, "custom-provider", "custom-model", "", "", false)

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
	assert.Equal(t, "group-1", item.ConcurrencyGroup)
	assert.Equal(t, true, item.CancelInProgress)
	assert.Equal(t, "custom-provider", item.AgentProvider)
	assert.Equal(t, "custom-model", item.AgentModel)
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

	submitAdHocJob(server.URL, "http://repo.com", "My Task", "", 0, 0, 0, nil, nil, nil, nil, nil, false, nil, nil, nil, "", false, "", "", "", "", false)

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

func TestPurgeJobsByTag(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/history", r.URL.Path)
		assert.Equal(t, "tag=my-tag", r.URL.RawQuery)
		assert.Equal(t, http.MethodDelete, r.Method)

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"cleared": 4}`))
	}))
	defer server.Close()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	purgeJobsByTag(server.URL, "my-tag")

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Successfully purged 4 jobs with tag 'my-tag'.")
}

func TestPurgeJobsByTag_ErrorResponse(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	purgeJobsByTag(server.URL, "error-tag")

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Failed to purge jobs by tag: Internal Server Error")
	assert.Equal(t, 1, exitCode)
}

func TestUpdateDependencies(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs/MY-JOB/dependencies", r.URL.Path)
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var reqBody map[string][]string
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)
		assert.Equal(t, []string{"DEP-1", "DEP-2"}, reqBody["depends_on"])

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success": true}`))
	}))
	defer server.Close()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	updateDependencies(server.URL, "MY-JOB", []string{"DEP-1", "DEP-2"})

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Job MY-JOB dependencies updated to: DEP-1, DEP-2")
}

func TestUpdateDependencies_ErrorResponse(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Bad Request"))
	}))
	defer server.Close()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	updateDependencies(server.URL, "MY-JOB", []string{"DEP-1"})

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Failed to update dependencies: Bad Request")
	assert.Equal(t, 1, exitCode)
}

func TestSetJobOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs/MY-JOB/output", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var reqBody map[string]map[string]string
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)
		assert.Equal(t, "some-value", reqBody["outputs"]["MY_KEY"])

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": true}`))
	}))
	defer server.Close()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	setJobOutput(server.URL, "MY-JOB", "MY_KEY", "some-value")

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Successfully set output MY_KEY=some-value for job MY-JOB")
}

func TestSetJobOutput_ErrorResponse(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Not Found"))
	}))
	defer server.Close()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	setJobOutput(server.URL, "UNKNOWN-JOB", "MY_KEY", "some-value")

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Failed to set job output: Not Found")
	assert.Equal(t, 1, exitCode)
}

func TestSetJobProgress(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs/MY-JOB/progress", r.URL.Path)
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var reqBody map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)

		progressVal, ok := reqBody["progress"].(float64)
		assert.True(t, ok)
		assert.Equal(t, 50.0, progressVal)

		msgVal, ok := reqBody["status_message"].(string)
		assert.True(t, ok)
		assert.Equal(t, "Halfway there", msgVal)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": true}`))
	}))
	defer server.Close()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	progress := 50
	msg := "Halfway there"
	setJobProgress(server.URL, "MY-JOB", &progress, &msg)

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Successfully updated progress for job MY-JOB")
}

func TestSetJobProgress_ErrorResponse(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	progress := 10
	setJobProgress(server.URL, "MY-JOB", &progress, nil)

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Failed to set job progress: Internal Server Error")
	assert.Equal(t, 1, exitCode)
}

func TestAddJobMetrics(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs/MY-JOB/metrics", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var reqBody map[string]map[string]float64
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)

		val, ok := reqBody["metrics"]["test-metric"]
		assert.True(t, ok)
		assert.Equal(t, 42.5, val)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": true}`))
	}))
	defer server.Close()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	addJobMetrics(server.URL, "MY-JOB", "test-metric", 42.5)

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Successfully added metric test-metric=42.50 for job MY-JOB")
}

func TestAddJobMetrics_ErrorResponse(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Not Found"))
	}))
	defer server.Close()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	addJobMetrics(server.URL, "MY-JOB", "test-metric", 42.5)

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Failed to add job metrics: Not Found")
	assert.Equal(t, 1, exitCode)
}

func TestEditJob_Success(t *testing.T) {
	// Setup mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			assert.Equal(t, "/jobs/MY-JOB", r.URL.Path)
			w.WriteHeader(http.StatusOK)
			// Return a dummy job object
			job := orchestrator.JobInfo{
				Status: "Pending",
				WorkItem: orchestrator.WorkItem{
					ID:      "MY-JOB",
					Summary: "Original Summary",
				},
			}
			json.NewEncoder(w).Encode(job)
		} else if r.Method == http.MethodPut {
			assert.Equal(t, "/jobs/MY-JOB", r.URL.Path)
			var item orchestrator.WorkItem
			err := json.NewDecoder(r.Body).Decode(&item)
			require.NoError(t, err)
			assert.Equal(t, "MY-JOB", item.ID)
			assert.Equal(t, "Edited Summary", item.Summary)

			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"success": true}`))
		} else {
			t.Fatalf("Unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	// Capture stdout
	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	// Mock the editor to just replace the summary
	t.Setenv("EDITOR", `sh -c 'sed -i.bak -e "s/\"summary\": \"Original Summary\"/\"summary\": \"Edited Summary\"/g" "$1"' _`)

	editJob(server.URL, "MY-JOB")

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Job MY-JOB updated successfully.")
}

func TestEditJob_NotPending(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs/MY-JOB", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		job := orchestrator.JobInfo{
			Status: "Running",
			WorkItem: orchestrator.WorkItem{
				ID: "MY-JOB",
			},
		}
		json.NewEncoder(w).Encode(job)
	}))
	defer server.Close()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	editJob(server.URL, "MY-JOB")

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Cannot edit job MY-JOB. It is currently Running.")
	assert.Equal(t, 1, exitCode)
}

func TestEditJob_IDChanged(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			job := orchestrator.JobInfo{
				Status: "Pending",
				WorkItem: orchestrator.WorkItem{
					ID:      "MY-JOB",
					Summary: "Original Summary",
				},
			}
			json.NewEncoder(w).Encode(job)
		} else {
			t.Fatalf("Unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	// Mock the editor to replace the ID
	t.Setenv("EDITOR", `sh -c 'sed -i.bak -e "s/\"id\": \"MY-JOB\"/\"id\": \"OTHER-JOB\"/g" "$1"' _`)

	editJob(server.URL, "MY-JOB")

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Error: You cannot change the Job ID during edit.")
	assert.Equal(t, 1, exitCode)
}

func TestCancelJobsByStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs", r.URL.Path)
		assert.Equal(t, "status=Pending", r.URL.RawQuery)
		assert.Equal(t, http.MethodDelete, r.Method)

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"canceled": 2}`))
	}))
	defer server.Close()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	cancelJobsByStatus(server.URL, "Pending")

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Successfully canceled 2 jobs with status 'Pending'.")
}

func TestCancelJobsByStatus_ErrorResponse(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	cancelJobsByStatus(server.URL, "Running")

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Failed to cancel jobs by status: Internal Server Error")
	assert.Equal(t, 1, exitCode)
}

func TestCancelJobsByMatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs", r.URL.Path)
		assert.Equal(t, "match=test", r.URL.RawQuery)
		assert.Equal(t, http.MethodDelete, r.Method)

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"canceled": 5}`))
	}))
	defer server.Close()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	cancelJobsByMatch(server.URL, "test")

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Successfully canceled 5 jobs matching 'test'.")
}

func TestCancelJobsByMatch_ErrorResponse(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	cancelJobsByMatch(server.URL, "error-match")

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Failed to cancel jobs by match: Internal Server Error")
	assert.Equal(t, 1, exitCode)
}

func TestCancelJobsByStatus_InvalidJSON(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	cancelJobsByStatus(server.URL, "Pending")

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Failed to decode response:")
	assert.Equal(t, 1, exitCode)
}

func TestCancelJobsByStatus_MissingField(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"other": 1}`))
	}))
	defer server.Close()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	cancelJobsByStatus(server.URL, "Pending")

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Unexpected response format")
	assert.Equal(t, 1, exitCode)
}

func TestCancelJobsByStatus_ConnectionError(t *testing.T) {
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

	cancelJobsByStatus("http://localhost:123456", "Pending")

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Failed to connect to orchestrator")
	assert.Equal(t, 1, exitCode)
}

func TestCancelJobsByMatch_InvalidJSON(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	cancelJobsByMatch(server.URL, "test")

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Failed to decode response:")
	assert.Equal(t, 1, exitCode)
}

func TestCancelJobsByMatch_MissingField(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"other": 1}`))
	}))
	defer server.Close()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	cancelJobsByMatch(server.URL, "test")

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Unexpected response format")
	assert.Equal(t, 1, exitCode)
}

func TestCancelJobsByMatch_ConnectionError(t *testing.T) {
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

	cancelJobsByMatch("http://localhost:123456", "test")

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Failed to connect to orchestrator")
	assert.Equal(t, 1, exitCode)
}

func TestPurgeJobsByStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/history", r.URL.Path)
		assert.Equal(t, "status=Completed", r.URL.RawQuery)
		assert.Equal(t, http.MethodDelete, r.Method)

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"cleared": 10}`))
	}))
	defer server.Close()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	purgeJobsByStatus(server.URL, "Completed")

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Successfully purged 10 jobs with status 'Completed'.")
}

func TestPurgeJobsByStatus_ErrorResponse(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	purgeJobsByStatus(server.URL, "Failed")

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Failed to purge jobs by status: Internal Server Error")
	assert.Equal(t, 1, exitCode)
}

func TestPurgeJobsByStatus_InvalidJSON(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	purgeJobsByStatus(server.URL, "Completed")

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Failed to decode response:")
	assert.Equal(t, 1, exitCode)
}

func TestPurgeJobsByStatus_MissingField(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"other": 1}`))
	}))
	defer server.Close()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	purgeJobsByStatus(server.URL, "Completed")

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Unexpected response format")
	assert.Equal(t, 1, exitCode)
}

func TestPurgeJobsByStatus_ConnectionError(t *testing.T) {
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

	purgeJobsByStatus("http://localhost:123456", "Completed")

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Failed to connect to orchestrator")
	assert.Equal(t, 1, exitCode)
}

func TestPurgeJobsByGroup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/history", r.URL.Path)
		assert.Equal(t, "group=test-group", r.URL.RawQuery)
		assert.Equal(t, http.MethodDelete, r.Method)

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"cleared": 3}`))
	}))
	defer server.Close()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
		pw.Close()
	}()

	purgeJobsByGroup(server.URL, "test-group")

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Successfully purged 3 jobs with concurrency group 'test-group'.")
}

func TestPurgeJobsByGroup_ErrorResponse(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
		pw.Close()
	}()

	purgeJobsByGroup(server.URL, "test-group")

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Failed to purge jobs by concurrency group: Internal Server Error")
	assert.Equal(t, 1, exitCode)
}

func TestPurgeJobsByGroup_InvalidJSON(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
		pw.Close()
	}()

	purgeJobsByGroup(server.URL, "test-group")

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Failed to parse response:")
	assert.Equal(t, 1, exitCode)
}

func TestPurgeJobsByGroup_MissingField(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"other": 1}`))
	}))
	defer server.Close()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
		pw.Close()
	}()

	purgeJobsByGroup(server.URL, "test-group")

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Unexpected response format")
	assert.Equal(t, 1, exitCode)
}

func TestPurgeJobsByGroup_ConnectionError(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
		pw.Close()
	}()

	purgeJobsByGroup("http://localhost:123456", "test-group")

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Failed to connect to orchestrator")
	assert.Equal(t, 1, exitCode)
}

func TestPurgeJobsByMatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/history", r.URL.Path)
		assert.Equal(t, "match=test", r.URL.RawQuery)
		assert.Equal(t, http.MethodDelete, r.Method)

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"cleared": 5}`))
	}))
	defer server.Close()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	purgeJobsByMatch(server.URL, "test")

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Successfully purged 5 jobs matching 'test'.")
}

func TestPurgeJobsByMatch_ErrorResponse(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	purgeJobsByMatch(server.URL, "error-match")

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Failed to purge jobs by match: Internal Server Error")
	assert.Equal(t, 1, exitCode)
}

func TestPurgeJobsByMatch_InvalidJSON(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	purgeJobsByMatch(server.URL, "test")

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Failed to decode response:")
	assert.Equal(t, 1, exitCode)
}

func TestPurgeJobsByMatch_MissingField(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"other": 1}`))
	}))
	defer server.Close()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	purgeJobsByMatch(server.URL, "test")

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Unexpected response format")
	assert.Equal(t, 1, exitCode)
}

func TestPurgeJobsByMatch_ConnectionError(t *testing.T) {
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

	purgeJobsByMatch("http://localhost:123456", "test")

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Failed to connect to orchestrator")
	assert.Equal(t, 1, exitCode)
}

func TestPurgeJobsByTag_InvalidJSON(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	purgeJobsByTag(server.URL, "test")

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Failed to decode response:")
	assert.Equal(t, 1, exitCode)
}

func TestPurgeJobsByTag_MissingField(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"other": 1}`))
	}))
	defer server.Close()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	purgeJobsByTag(server.URL, "test")

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Unexpected response format")
	assert.Equal(t, 1, exitCode)
}

func TestPurgeJobsByTag_ConnectionError(t *testing.T) {
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

	purgeJobsByTag("http://localhost:123456", "test")

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Failed to connect to orchestrator")
	assert.Equal(t, 1, exitCode)
}

func TestCloneJob_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs/ORIG-JOB/clone", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		w.WriteHeader(http.StatusAccepted)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"cloned_job_id": "NEW-JOB-123"}`))
	}))
	defer server.Close()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	priority := 10
	cloneJob(server.URL, "ORIG-JOB", "NEW-JOB-123", &priority, false, map[string]string{"K": "V"}, []string{"DEP1"})

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Job ORIG-JOB cloned successfully as NEW-JOB-123")
}

func TestCloneJob_ConnectionError(t *testing.T) {
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

	cloneJob("http://localhost:123456", "ORIG-JOB", "", nil, false, nil, nil)

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Failed to connect to orchestrator")
	assert.Equal(t, 1, exitCode)
}

func TestCloneJob_ErrorResponse(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Error"))
	}))
	defer server.Close()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	cloneJob(server.URL, "ORIG-JOB", "", nil, false, nil, nil)

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Failed to clone job: Internal Error")
	assert.Equal(t, 1, exitCode)
}

func TestCloneJob_InvalidJSON(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	cloneJob(server.URL, "ORIG-JOB", "", nil, false, nil, nil)

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Failed to parse response")
	assert.Equal(t, 1, exitCode)
}

func TestCloneJob_WaitFailed(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusAccepted)
			w.Write([]byte(`{"cloned_job_id": "NEW-JOB-WAIT"}`))
		} else if r.Method == http.MethodGet {
			callCount++
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": "Failed", "error": "test failure"}`))
		}
	}))
	defer server.Close()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	cloneJob(server.URL, "ORIG-JOB", "", nil, true, nil, nil)

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Job failed: job failed with error: test failure")
	assert.Equal(t, 1, exitCode)
	assert.Greater(t, callCount, 0)
}

func TestSubmitMatrixJob_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs/matrix", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		w.WriteHeader(http.StatusAccepted)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"submitted": ["JOB-1", "JOB-2"], "errors": []}`))
	}))
	defer server.Close()

	f, _ := os.CreateTemp("", "matrix_*.json")
	f.Write([]byte(`{"matrix": {}}`))
	f.Close()
	defer os.Remove(f.Name())

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	submitMatrixJob(server.URL, f.Name(), false)

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Matrix submission completed.")
	assert.Contains(t, string(out), "Successfully submitted jobs: JOB-1, JOB-2")
}

func TestSubmitMatrixJob_InvalidFile(t *testing.T) {
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

	submitMatrixJob("http://localhost", "non_existent_file.json", false)

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Failed to open file non_existent_file.json")
	assert.Equal(t, 1, exitCode)
}

func TestSubmitMatrixJob_ConnectionError(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	f, _ := os.CreateTemp("", "matrix_*.json")
	f.Write([]byte(`{"matrix": {}}`))
	f.Close()
	defer os.Remove(f.Name())

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	submitMatrixJob("http://localhost:123456", f.Name(), false)

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Failed to connect to orchestrator")
	assert.Equal(t, 1, exitCode)
}

func TestSubmitMatrixJob_ErrorResponse(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	f, _ := os.CreateTemp("", "matrix_*.json")
	f.Write([]byte(`{"matrix": {}}`))
	f.Close()
	defer os.Remove(f.Name())

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	submitMatrixJob(server.URL, f.Name(), false)

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Failed to submit matrix job: Internal Server Error")
	assert.Equal(t, 1, exitCode)
}

func TestSubmitMatrixJob_InvalidJSON(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	f, _ := os.CreateTemp("", "matrix_*.json")
	f.Write([]byte(`{"matrix": {}}`))
	f.Close()
	defer os.Remove(f.Name())

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	submitMatrixJob(server.URL, f.Name(), false)

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Failed to parse matrix response")
	assert.Equal(t, 1, exitCode)
}

func TestSubmitMatrixJob_WithErrors(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{"submitted": ["JOB-1"], "errors": ["some error"]}`))
	}))
	defer server.Close()

	f, _ := os.CreateTemp("", "matrix_*.json")
	f.Write([]byte(`{"matrix": {}}`))
	f.Close()
	defer os.Remove(f.Name())

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	submitMatrixJob(server.URL, f.Name(), false)

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Successfully submitted jobs: JOB-1")
	assert.Contains(t, string(out), "Errors:")
	assert.Contains(t, string(out), "some error")
	assert.Equal(t, 0, exitCode) // still 0 if some were submitted
}

func TestSubmitMatrixJob_OnlyErrors(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{"submitted": [], "errors": ["all failed"]}`))
	}))
	defer server.Close()

	f, _ := os.CreateTemp("", "matrix_*.json")
	f.Write([]byte(`{"matrix": {}}`))
	f.Close()
	defer os.Remove(f.Name())

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	submitMatrixJob(server.URL, f.Name(), false)

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Errors:")
	assert.Contains(t, string(out), "all failed")
	assert.Equal(t, 1, exitCode)
}

func TestSubmitMatrixJob_WaitFailed(t *testing.T) {
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
			w.Write([]byte(`{"status": "Failed", "error": "test matrix failure"}`))
		}
	}))
	defer server.Close()

	f, _ := os.CreateTemp("", "matrix_*.json")
	f.Write([]byte(`{"matrix": {}}`))
	f.Close()
	defer os.Remove(f.Name())

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	submitMatrixJob(server.URL, f.Name(), true)

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "job JOB-WAIT failed with error: test matrix failure")
	assert.Equal(t, 1, exitCode)
	assert.Greater(t, callCount, 0)
}

func TestEditJob_ConnectionErrorGET(t *testing.T) {
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

	editJob("http://localhost:123456", "MY-JOB")

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Failed to connect to orchestrator")
	assert.Equal(t, 1, exitCode)
}

func TestEditJob_ErrorResponseGET(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Not Found"))
	}))
	defer server.Close()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	editJob(server.URL, "MY-JOB")

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Failed to fetch job details: Not Found")
	assert.Equal(t, 1, exitCode)
}

func TestEditJob_InvalidJSONGET(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	editJob(server.URL, "MY-JOB")

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Failed to decode job response")
	assert.Equal(t, 1, exitCode)
}

func TestEditJob_EditorError(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		job := orchestrator.JobInfo{
			Status: "Pending",
			WorkItem: orchestrator.WorkItem{
				ID:      "MY-JOB",
				Summary: "Original Summary",
			},
		}
		json.NewEncoder(w).Encode(job)
	}))
	defer server.Close()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	t.Setenv("EDITOR", "false") // false returns exit code 1

	editJob(server.URL, "MY-JOB")

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Editor exited with error")
	assert.Equal(t, 1, exitCode)
}

func TestEditJob_InvalidJSONAfterEdit(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		job := orchestrator.JobInfo{
			Status: "Pending",
			WorkItem: orchestrator.WorkItem{
				ID:      "MY-JOB",
				Summary: "Original Summary",
			},
		}
		json.NewEncoder(w).Encode(job)
	}))
	defer server.Close()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	t.Setenv("EDITOR", `sh -c 'echo "invalid json" > "$1"' _`)

	editJob(server.URL, "MY-JOB")

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Failed to parse modified JSON")
	assert.Equal(t, 1, exitCode)
}

func TestEditJob_ErrorResponsePUT(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			job := orchestrator.JobInfo{
				Status: "Pending",
				WorkItem: orchestrator.WorkItem{
					ID:      "MY-JOB",
					Summary: "Original Summary",
				},
			}
			json.NewEncoder(w).Encode(job)
		} else if r.Method == http.MethodPut {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal Server Error"))
		}
	}))
	defer server.Close()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	t.Setenv("EDITOR", `sh -c 'sed -i.bak -e "s/\"summary\": \"Original Summary\"/\"summary\": \"Edited Summary\"/g" "$1"' _`)
	t.Setenv("EDITOR", `sh -c 'curl -X POST -s http://localhost:0/kill || true && sed -i.bak -e "s/\"summary\": \"Original Summary\"/\"summary\": \"Edited Summary\"/g" "$1"' _`)

	editJob(server.URL, "MY-JOB")

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Failed to update job: Internal Server Error")
	assert.Equal(t, 1, exitCode)
}

func TestEditJob_ConnectionErrorPUT(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	// We only mock the GET part, then kill the server so PUT fails

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			job := orchestrator.JobInfo{
				Status: "Pending",
				WorkItem: orchestrator.WorkItem{
					ID:      "MY-JOB",
					Summary: "Original Summary",
				},
			}
			json.NewEncoder(w).Encode(job)
		}
	}))

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	t.Setenv("EDITOR", `sh -c 'curl -X POST -s http://localhost:0/kill || true && sed -e "s/\"summary\": \"Original Summary\"/\"summary\": \"Edited Summary\"/g" "$1" > "$1.tmp" && mv "$1.tmp" "$1"' _`)

	// Close server before edit finishes if we could, but instead let's just make the editor script break the URL somehow,
	// actually the easiest way to test connection error on PUT is just to mock HTTP client, but since we use DefaultClient,
	// let's do a different trick: we can use a httptest.Server that closes the connection on PUT.
	server.Close()

	// Wait, we need the server open for GET. Let's start a new server that handles GET, then closes itself.
	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			job := orchestrator.JobInfo{
				Status: "Pending",
				WorkItem: orchestrator.WorkItem{
					ID:      "MY-JOB",
					Summary: "Original Summary",
				},
			}
			json.NewEncoder(w).Encode(job)
		} else if r.Method == http.MethodPut {
			// Instead of closing, panic or hijack and close.
			hj, _ := w.(http.Hijacker)
			conn, _, _ := hj.Hijack()
			conn.Close()
		}
	}))
	defer server2.Close()

	t.Setenv("EDITOR", `sh -c 'sed -i.bak -e "s/\"summary\": \"Original Summary\"/\"summary\": \"Edited Summary\"/g" "$1"' _`)

	editJob(server2.URL, "MY-JOB")

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Failed to connect to orchestrator")
	assert.Equal(t, 1, exitCode)
}

// The original submission_clone_test.go had a payload check for the Clone. Adding it here to replace what was deleted.
func TestCloneJob_PayloadCheck(t *testing.T) {
	var receivedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs/ORIG-JOB/clone", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		receivedBody, _ = io.ReadAll(r.Body)

		w.WriteHeader(http.StatusAccepted)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"cloned_job_id": "NEW-JOB-123"}`))
	}))
	defer server.Close()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	priority := 10
	cloneJob(server.URL, "ORIG-JOB", "NEW-JOB-123", &priority, false, map[string]string{"K": "V"}, []string{"DEP1"})

	pw.Close()
	io.ReadAll(pr) // ignore stdout

	var payload map[string]interface{}
	err := json.Unmarshal(receivedBody, &payload)
	require.NoError(t, err)

	assert.Equal(t, "NEW-JOB-123", payload["new_id"])

	// Convert types to check properly
	envVars, ok := payload["env_vars"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "V", envVars["K"])

	pri, ok := payload["priority"].(float64)
	assert.True(t, ok)
	assert.Equal(t, float64(10), pri)

	deps, ok := payload["depends_on"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, deps, 1)
	assert.Equal(t, "DEP1", deps[0])
}

func TestPurgeJobsByTag_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/history", r.URL.Path)
		assert.Equal(t, "tag=my-tag", r.URL.RawQuery)
		assert.Equal(t, http.MethodDelete, r.Method)

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"cleared": 4}`))
	}))
	defer server.Close()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	purgeJobsByTag(server.URL, "my-tag")

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Successfully purged 4 jobs with tag 'my-tag'.")
}

func TestCloneBulkJobs_ConnectionError(t *testing.T) {
	oldExit := exitFunc
	exitCalled := false
	exitFunc = func(code int) {
		exitCalled = true
	}
	defer func() { exitFunc = oldExit }()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	cloneBulkJobs("http://localhost:12345", ".*", "tag-1", nil, false, nil, nil, false)
	pw.Close()

	output, _ := io.ReadAll(pr)
	assert.Contains(t, string(output), "Failed to connect to orchestrator")
	assert.True(t, exitCalled)
}

func TestCloneBulkJobs_InvalidURL(t *testing.T) {
	oldExit := exitFunc
	exitCalled := false
	exitFunc = func(code int) {
		exitCalled = true
	}
	defer func() { exitFunc = oldExit }()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	cloneBulkJobs("://invalid-url", ".*", "tag-1", nil, false, nil, nil, false)
	pw.Close()

	output, _ := io.ReadAll(pr)
	assert.Contains(t, string(output), "Failed to parse URL")
	assert.True(t, exitCalled)
}

func TestCloneBulkJobs_InvalidJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte("{invalid-json}"))
	}))
	defer server.Close()

	oldExit := exitFunc
	exitCalled := false
	exitFunc = func(code int) {
		exitCalled = true
	}
	defer func() { exitFunc = oldExit }()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	cloneBulkJobs(server.URL, ".*", "tag-1", nil, false, nil, nil, false)
	pw.Close()

	output, _ := io.ReadAll(pr)
	assert.Contains(t, string(output), "Failed to parse response")
	assert.True(t, exitCalled)
}

func TestUpdateDependencies_ConnectionError(t *testing.T) {
	oldExit := exitFunc
	exitCalled := false
	exitFunc = func(code int) { exitCalled = true }
	defer func() { exitFunc = oldExit }()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() { stdout = oldStdout }()

	updateDependencies("http://localhost:12345", "MY-JOB", []string{"DEP-1"})
	pw.Close()

	out, _ := io.ReadAll(pr)
	assert.Contains(t, string(out), "Failed to connect to orchestrator")
	assert.True(t, exitCalled)
}

func TestUpdateAgent_ConnectionError(t *testing.T) {
	oldExit := exitFunc
	exitCalled := false
	exitFunc = func(code int) { exitCalled = true }
	defer func() { exitFunc = oldExit }()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() { stdout = oldStdout }()

	updateAgent("http://localhost:12345", "MY-JOB", "provider", "model")
	pw.Close()

	out, _ := io.ReadAll(pr)
	assert.Contains(t, string(out), "Failed to connect to orchestrator")
	assert.True(t, exitCalled)
}

func TestSetJobOutput_ConnectionError(t *testing.T) {
	oldExit := exitFunc
	exitCalled := false
	exitFunc = func(code int) { exitCalled = true }
	defer func() { exitFunc = oldExit }()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() { stdout = oldStdout }()

	setJobOutput("http://localhost:12345", "MY-JOB", "key", "val")
	pw.Close()

	out, _ := io.ReadAll(pr)
	assert.Contains(t, string(out), "Failed to connect to orchestrator")
	assert.True(t, exitCalled)
}

func TestSetJobProgress_ConnectionError(t *testing.T) {
	oldExit := exitFunc
	exitCalled := false
	exitFunc = func(code int) { exitCalled = true }
	defer func() { exitFunc = oldExit }()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() { stdout = oldStdout }()

	prog := 50
	msg := "Halfway there"
	setJobProgress("http://localhost:12345", "MY-JOB", &prog, &msg)
	pw.Close()

	out, _ := io.ReadAll(pr)
	assert.Contains(t, string(out), "Failed to connect to orchestrator")
	assert.True(t, exitCalled)
}

func TestAddJobMetrics_ConnectionError(t *testing.T) {
	oldExit := exitFunc
	exitCalled := false
	exitFunc = func(code int) { exitCalled = true }
	defer func() { exitFunc = oldExit }()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() { stdout = oldStdout }()

	addJobMetrics("http://localhost:12345", "MY-JOB", "key", 1.5)
	pw.Close()

	out, _ := io.ReadAll(pr)
	assert.Contains(t, string(out), "Failed to connect to orchestrator")
	assert.True(t, exitCalled)
}

func TestUnholdJobs_ConnectionError(t *testing.T) {
	oldExit := exitFunc
	exitCalled := false
	exitFunc = func(code int) { exitCalled = true }
	defer func() { exitFunc = oldExit }()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() { stdout = oldStdout }()

	unholdJobs("http://localhost:12345", "match-val", "tag-val")
	pw.Close()

	out, _ := io.ReadAll(pr)
	assert.Contains(t, string(out), "Failed to connect to orchestrator")
	assert.True(t, exitCalled)
}

func TestRetryEditJob_Success(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/jobs/MY-JOB" {
			w.WriteHeader(http.StatusOK)
			job := orchestrator.JobInfo{
				Status: "Failed",
				WorkItem: orchestrator.WorkItem{
					ID:      "MY-JOB",
					Summary: "Original Summary",
				},
			}
			json.NewEncoder(w).Encode(job)
		} else if r.Method == http.MethodPost && r.URL.Path == "/jobs" {
			var item orchestrator.WorkItem
			json.NewDecoder(r.Body).Decode(&item)
			assert.Equal(t, "MY-JOB-retry", item.ID)
			assert.Equal(t, "Edited Summary", item.Summary)
			w.WriteHeader(http.StatusAccepted)
			w.Write([]byte("Job MY-JOB-retry accepted"))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	t.Setenv("EDITOR", `sh -c 'sed -i.bak -e "s/\"summary\": \"Original Summary\"/\"summary\": \"Edited Summary\"/g" "$1"' _`)
	t.Setenv("EDITOR", `sh -c 'sed -i.bak -e "s/\"summary\": \"Original Summary\"/\"summary\": \"Edited Summary\"/g" "$1"' _`)
	t.Setenv("EDITOR", `sh -c 'sed -i.bak -e "s/\"summary\": \"Original Summary\"/\"summary\": \"Edited Summary\"/g" "$1"' _`)

	retryEditJob(server.URL, "MY-JOB", false)

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Job MY-JOB-retry submitted successfully.")
	assert.Equal(t, 0, exitCode)
}

func TestRetryEditJob_ConnectionErrorPOST(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/jobs/MY-JOB" {
			w.WriteHeader(http.StatusOK)
			job := orchestrator.JobInfo{
				Status: "Failed",
				WorkItem: orchestrator.WorkItem{
					ID:      "MY-JOB",
					Summary: "Original Summary",
				},
			}
			json.NewEncoder(w).Encode(job)
		} else {
			// hijack connection on POST to simulate error
			if hj, ok := w.(http.Hijacker); ok {
				conn, _, _ := hj.Hijack()
				conn.Close()
			}
		}
	}))
	defer server.Close()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	t.Setenv("EDITOR", `sh -c 'sed -e "s/\"summary\": \"Original Summary\"/\"summary\": \"Edited Summary\"/g" "$1" > "$1.tmp" && mv "$1.tmp" "$1"' _`)

	retryEditJob(server.URL, "MY-JOB", false)

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Failed to connect to orchestrator at")
	assert.Equal(t, 1, exitCode)
}

func TestRetryEditJob_ErrorResponsePOST(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/jobs/MY-JOB" {
			w.WriteHeader(http.StatusOK)
			job := orchestrator.JobInfo{
				Status: "Failed",
				WorkItem: orchestrator.WorkItem{
					ID:      "MY-JOB",
					Summary: "Original Summary",
				},
			}
			json.NewEncoder(w).Encode(job)
		} else if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("invalid request"))
		}
	}))
	defer server.Close()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	t.Setenv("EDITOR", `sh -c 'sed -e "s/\"summary\": \"Original Summary\"/\"summary\": \"Edited Summary\"/g" "$1" > "$1.tmp" && mv "$1.tmp" "$1"' _`)

	retryEditJob(server.URL, "MY-JOB", false)

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Failed to submit retried job: invalid request")
	assert.Equal(t, 1, exitCode)
}

func TestRetryEditJob_WaitError(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/jobs/MY-JOB" {
			w.WriteHeader(http.StatusOK)
			job := orchestrator.JobInfo{
				Status: "Failed",
				WorkItem: orchestrator.WorkItem{
					ID:      "MY-JOB",
					Summary: "Original Summary",
				},
			}
			json.NewEncoder(w).Encode(job)
		} else if r.Method == http.MethodPost && r.URL.Path == "/jobs" {
			w.WriteHeader(http.StatusAccepted)
		} else if r.Method == http.MethodGet && r.URL.Path == "/jobs/MY-JOB-retry" {
			callCount++
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": "Failed", "error": "test failure"}`))
		}
	}))
	defer server.Close()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	t.Setenv("EDITOR", `sh -c 'sed -i.bak -e "s/\"summary\": \"Original Summary\"/\"summary\": \"Edited Summary\"/g" "$1"' _`)

	retryEditJob(server.URL, "MY-JOB", true)

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Job failed: job failed with error: test failure")
	assert.Equal(t, 1, exitCode)
	assert.Greater(t, callCount, 0)
}

func TestRenameJob_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs/JOB-123/rename", r.URL.Path)
		assert.Equal(t, http.MethodPut, r.Method)

		body, _ := io.ReadAll(r.Body)
		assert.Contains(t, string(body), `{"new_id": "NEW-JOB-123"}`)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	var buf bytes.Buffer
	originalStdout := stdout
	stdout = &buf
	defer func() { stdout = originalStdout }()

	renameJob(server.URL, "JOB-123", "NEW-JOB-123")

	assert.Contains(t, buf.String(), "Job JOB-123 renamed successfully to NEW-JOB-123")
}

func TestRenameJob_Failure(t *testing.T) {
	if os.Getenv("CRASH_TEST") == "1" {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("internal error"))
		}))
		defer server.Close()

		// Temporarily replace exitFunc to just print and exit 1
		exitFunc = func(code int) {
			os.Exit(code)
		}

		renameJob(server.URL, "JOB-123", "NEW-JOB-123")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestRenameJob_Failure")
	cmd.Env = append(os.Environ(), "CRASH_TEST=1")

	var stdoutBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf

	err := cmd.Run()
	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		assert.Contains(t, stdoutBuf.String(), "Failed to rename job: internal error")
		return
	}
	t.Fatalf("process ran with err %v, want exit status 1", err)
}

func TestRenameJob_ConnectionError(t *testing.T) {
	if os.Getenv("CRASH_TEST_RENAME_CONN") == "1" {
		// Temporarily replace exitFunc to just print and exit 1
		exitFunc = func(code int) {
			os.Exit(code)
		}

		renameJob("http://localhost:0", "JOB-123", "NEW-JOB-123")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestRenameJob_ConnectionError")
	cmd.Env = append(os.Environ(), "CRASH_TEST_RENAME_CONN=1")

	var stdoutBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf

	err := cmd.Run()
	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		assert.Contains(t, stdoutBuf.String(), "Failed to connect to orchestrator")
		return
	}
	t.Fatalf("process ran with err %v, want exit status 1", err)
}

func TestSkipJob_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs/JOB-123/skip", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	var buf bytes.Buffer
	originalStdout := stdout
	stdout = &buf
	defer func() { stdout = originalStdout }()

	skipJob(server.URL, "JOB-123")

	assert.Contains(t, buf.String(), "Job JOB-123 skipped successfully")
}

func TestSkipJobs_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs/skip", r.URL.Path)
		assert.Equal(t, "tag=test", r.URL.RawQuery)
		assert.Equal(t, http.MethodPost, r.Method)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"skipped": 2}`))
	}))
	defer server.Close()

	var buf bytes.Buffer
	originalStdout := stdout
	stdout = &buf
	defer func() { stdout = originalStdout }()

	skipJobs(server.URL, "", "test", "")

	assert.Contains(t, buf.String(), "Successfully skipped 2 jobs")
}

func TestSkipJobs_Failure(t *testing.T) {
	if os.Getenv("CRASH_TEST") == "1" {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("internal error"))
		}))
		defer server.Close()

		exitFunc = func(code int) {
			os.Exit(code)
		}

		skipJobs(server.URL, "", "test", "")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestSkipJobs_Failure")
	cmd.Env = append(os.Environ(), "CRASH_TEST=1")

	var stdoutBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf

	err := cmd.Run()
	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		assert.Contains(t, stdoutBuf.String(), "Failed to skip jobs: internal error")
		return
	}
	t.Fatalf("process ran with err %v, want exit status 1", err)
}

func TestSkipJobs_ConnectionError(t *testing.T) {
	if os.Getenv("CRASH_TEST") == "1" {
		exitFunc = func(code int) {
			os.Exit(code)
		}

		skipJobs("http://localhost:12345", "", "test", "")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestSkipJobs_ConnectionError")
	cmd.Env = append(os.Environ(), "CRASH_TEST=1")

	var stdoutBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf

	err := cmd.Run()
	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		assert.Contains(t, stdoutBuf.String(), "Failed to connect to orchestrator")
		return
	}
	t.Fatalf("process ran with err %v, want exit status 1", err)
}

func TestSkipJob_ConnectionError(t *testing.T) {
	if os.Getenv("CRASH_TEST") == "1" {
		exitFunc = func(code int) {
			os.Exit(code)
		}

		skipJob("http://localhost:12345", "test")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestSkipJob_ConnectionError")
	cmd.Env = append(os.Environ(), "CRASH_TEST=1")

	var stdoutBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf

	err := cmd.Run()
	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		assert.Contains(t, stdoutBuf.String(), "Failed to connect to orchestrator")
		return
	}
	t.Fatalf("process ran with err %v, want exit status 1", err)
}

func TestSkipJob_Failure(t *testing.T) {
	if os.Getenv("CRASH_TEST") == "1" {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("internal error"))
		}))
		defer server.Close()

		exitFunc = func(code int) {
			os.Exit(code)
		}

		skipJob(server.URL, "JOB-123")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestSkipJob_Failure")
	cmd.Env = append(os.Environ(), "CRASH_TEST=1")

	var stdoutBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf

	err := cmd.Run()
	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		assert.Contains(t, stdoutBuf.String(), "Failed to skip job: internal error")
		return
	}
	t.Fatalf("process ran with err %v, want exit status 1", err)
}

func TestImportPipelineJob(t *testing.T) {
	tests := []struct {
		name           string
		setupServer    func() *httptest.Server
		filePath       string
		target         string
		vars           map[string]string
		expectedOutput string
		expectedExit   int
		setupFile      func() string
	}{
		{
			name: "Success with target and vars",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "/jobs/pipeline/import", r.URL.Path)
					assert.Equal(t, http.MethodPost, r.Method)
					assert.Equal(t, "my-target", r.URL.Query().Get("target"))
					assert.Equal(t, "var1=val1", r.URL.Query().Get("var"))

					w.WriteHeader(http.StatusAccepted)
					w.Write([]byte(`{"submitted": ["JOB-1", "JOB-2"], "errors": []}`))
				}))
			},
			target:         "my-target",
			vars:           map[string]string{"var1": "val1"},
			expectedOutput: "Successfully imported 2 jobs:\n - JOB-1\n - JOB-2",
			expectedExit:   0,
			setupFile: func() string {
				f, _ := os.CreateTemp("", "pipeline_*.yaml")
				f.Write([]byte(`name: my-pipeline`))
				f.Close()
				return f.Name()
			},
		},
		{
			name: "Success with only submitted",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{"submitted": ["JOB-1"]}`))
				}))
			},
			expectedOutput: "Successfully imported 1 jobs:\n - JOB-1",
			expectedExit:   0,
			setupFile: func() string {
				f, _ := os.CreateTemp("", "pipeline_*.yaml")
				f.Write([]byte(`name: my-pipeline`))
				f.Close()
				return f.Name()
			},
		},
		{
			name: "Success with errors",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusAccepted)
					w.Write([]byte(`{"submitted": ["JOB-1"], "errors": ["error message"]}`))
				}))
			},
			expectedOutput: "Successfully imported 1 jobs:\n - JOB-1\n\nFailed to import 1 jobs:\n - error message",
			expectedExit:   1,
			setupFile: func() string {
				f, _ := os.CreateTemp("", "pipeline_*.yaml")
				f.Write([]byte(`name: my-pipeline`))
				f.Close()
				return f.Name()
			},
		},
		{
			name: "Only errors",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusAccepted)
					w.Write([]byte(`{"errors": ["fatal error"]}`))
				}))
			},
			expectedOutput: "\nFailed to import 1 jobs:\n - fatal error",
			expectedExit:   1,
			setupFile: func() string {
				f, _ := os.CreateTemp("", "pipeline_*.yaml")
				f.Write([]byte(`name: my-pipeline`))
				f.Close()
				return f.Name()
			},
		},
		{
			name: "Invalid JSON response",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`invalid json`))
				}))
			},
			expectedOutput: "Pipeline successfully imported, but failed to parse response.",
			expectedExit:   0,
			setupFile: func() string {
				f, _ := os.CreateTemp("", "pipeline_*.yaml")
				f.Write([]byte(`name: my-pipeline`))
				f.Close()
				return f.Name()
			},
		},
		{
			name: "HTTP Failure (500)",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte(`internal server error`))
				}))
			},
			expectedOutput: "Failed to import pipeline: internal server error",
			expectedExit:   1,
			setupFile: func() string {
				f, _ := os.CreateTemp("", "pipeline_*.yaml")
				f.Write([]byte(`name: my-pipeline`))
				f.Close()
				return f.Name()
			},
		},
		{
			name: "Connection error",
			setupServer: func() *httptest.Server {
				s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
				s.Close()
				return s
			},
			expectedOutput: "Failed to connect to orchestrator",
			expectedExit:   1,
			setupFile: func() string {
				f, _ := os.CreateTemp("", "pipeline_*.yaml")
				f.Write([]byte(`name: my-pipeline`))
				f.Close()
				return f.Name()
			},
		},
		{
			name: "Invalid file path",
			setupServer: func() *httptest.Server {
				return nil
			},
			expectedOutput: "Failed to read file nonexistent.yaml",
			expectedExit:   1,
			filePath:       "nonexistent.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var exitCode int
			oldExit := exitFunc
			exitFunc = func(code int) { exitCode = code }
			defer func() { exitFunc = oldExit }()

			var server *httptest.Server
			var url string
			if tt.setupServer != nil {
				server = tt.setupServer()
				if server != nil {
					url = server.URL
				}
			}
			if server != nil && tt.name != "Connection error" {
				defer server.Close()
			}

			filePath := tt.filePath
			if tt.setupFile != nil {
				filePath = tt.setupFile()
				defer os.Remove(filePath)
			}

			oldStdout := stdout
			pr, pw, _ := os.Pipe()
			stdout = pw
			defer func() { stdout = oldStdout }()

			importPipelineJob(url, filePath, tt.target, tt.vars)

			pw.Close()
			outBytes, _ := io.ReadAll(pr)
			outStr := string(outBytes)

			assert.Contains(t, outStr, tt.expectedOutput)
			assert.Equal(t, tt.expectedExit, exitCode)
		})
	}
}

func TestSubmitMatrixInlineJob_Errors(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) {
		exitCode = code
	}
	defer func() {
		exitFunc = oldExit
	}()

	oldStdout := stdout
	_, w, _ := os.Pipe()
	stdout = w
	defer func() {
		stdout = oldStdout
	}()

	matrix := map[string][]string{
		"OS": {"linux"},
	}

	t.Run("Server Unreachable", func(t *testing.T) {
		exitCode = 0
		submitMatrixInlineJob("http://127.0.0.1:0", "http://repo.com", "task", "", 0, 0, 0, nil, nil, nil, nil, nil, false, nil, nil, nil, "", false, "", "", "", "", false, matrix)
		assert.Equal(t, 1, exitCode)
	})

	t.Run("Non-202 Accepted", func(t *testing.T) {
		exitCode = 0
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer ts.Close()
		submitMatrixInlineJob(ts.URL, "http://repo.com", "task", "", 0, 0, 0, nil, nil, nil, nil, nil, false, nil, nil, nil, "", false, "", "", "", "", false, matrix)
		assert.Equal(t, 1, exitCode)
	})

	t.Run("Invalid JSON Response", func(t *testing.T) {
		exitCode = 0
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusAccepted)
			w.Write([]byte(`{invalid json}`))
		}))
		defer ts.Close()
		submitMatrixInlineJob(ts.URL, "http://repo.com", "task", "", 0, 0, 0, nil, nil, nil, nil, nil, false, nil, nil, nil, "", false, "", "", "", "", false, matrix)
		assert.Equal(t, 1, exitCode)
	})

	t.Run("Errors In Payload", func(t *testing.T) {
		exitCode = 0
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusAccepted)
			w.Write([]byte(`{"submitted": [], "errors": ["some error"]}`))
		}))
		defer ts.Close()
		submitMatrixInlineJob(ts.URL, "http://repo.com", "task", "", 0, 0, 0, nil, nil, nil, nil, nil, false, nil, nil, nil, "", false, "", "", "", "", false, matrix)
		assert.Equal(t, 1, exitCode)
	})

	t.Run("Wait For Jobs", func(t *testing.T) {
		exitCode = 0
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/jobs/matrix" {
				w.WriteHeader(http.StatusAccepted)
				w.Write([]byte(`{"submitted": ["JOB-WAIT"], "errors": []}`))
			} else {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"id": "JOB-WAIT", "status": "Completed"}`))
			}
		}))
		defer ts.Close()
		submitMatrixInlineJob(ts.URL, "http://repo.com", "task", "", 0, 0, 0, nil, nil, nil, nil, nil, true, nil, nil, nil, "", false, "", "", "", "", false, matrix)
		assert.Equal(t, 0, exitCode)
	})

	t.Run("Wait For Jobs Failed", func(t *testing.T) {
		exitCode = 0
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/jobs/matrix" {
				w.WriteHeader(http.StatusAccepted)
				w.Write([]byte(`{"submitted": ["JOB-WAIT"], "errors": []}`))
			} else {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"id": "JOB-WAIT", "status": "Failed", "error": "simulated"}`))
			}
		}))
		defer ts.Close()
		submitMatrixInlineJob(ts.URL, "http://repo.com", "task", "", 0, 0, 0, nil, nil, nil, nil, nil, true, nil, nil, nil, "", false, "", "", "", "", false, matrix)
		assert.Equal(t, 1, exitCode)
	})
}

func TestWaitIdle(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/status" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"active": 0, "pending": 0}`))
		}
	}))
	defer ts.Close()

	var buf bytes.Buffer
	err := waitIdle(ts.URL, &buf)
	assert.NoError(t, err)
}

func TestWaitIdle_NeedsWait(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"active": 0, "pending": 0}`))
	}))
	defer ts.Close()

	var buf bytes.Buffer
	err := waitIdle(ts.URL, &buf)
	assert.NoError(t, err)
}

func TestWaitIdle_ErrorRecovery(t *testing.T) {
	var requests int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqCount := atomic.AddInt32(&requests, 1)

		if r.URL.Path == "/status" {
			if reqCount < 2 {
				w.WriteHeader(http.StatusInternalServerError)
			} else {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"active": 0, "pending": 0}`))
			}
		}
	}))
	defer ts.Close()

	var buf bytes.Buffer
	err := waitIdle(ts.URL, &buf)
	assert.NoError(t, err)
}

func TestGetJobMetrics(t *testing.T) {
	tests := []struct {
		name           string
		url            string
		handler        http.HandlerFunc
		key            string
		expectedOutput string
		expectedExit   int
	}{
		{
			name: "Success",
			key:  "cost",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/jobs/test-job", r.URL.Path)
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"id": "test-job", "metrics": {"cost": 42.5}}`))
			},
			expectedOutput: "42.5",
			expectedExit:   0,
		},
		{
			name: "MissingKey",
			key:  "missing",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/jobs/test-job", r.URL.Path)
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"id": "test-job", "metrics": {"cost": 42.5}}`))
			},
			expectedOutput: "Metrics key 'missing' not found for job test-job",
			expectedExit:   1,
		},
		{
			name: "ErrorResponse",
			key:  "cost",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/jobs/test-job", r.URL.Path)
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte(`Job not found`))
			},
			expectedOutput: "Failed to get job info",
			expectedExit:   1,
		},
		{
			name:           "ConnectionError",
			url:            "http://invalid-url",
			key:            "cost",
			expectedOutput: "Failed to connect to orchestrator",
			expectedExit:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var serverURL string
			if tt.handler != nil {
				server := httptest.NewServer(tt.handler)
				defer server.Close()
				serverURL = server.URL
			} else {
				serverURL = tt.url
			}

			oldStdout := stdout
			var buf bytes.Buffer
			stdout = &buf
			defer func() { stdout = oldStdout }()

			oldExit := exitFunc
			var exitCode int
			exitFunc = func(code int) { exitCode = code }
			defer func() { exitFunc = oldExit }()

			getJobMetrics(serverURL, "test-job", tt.key)

			assert.Contains(t, buf.String(), tt.expectedOutput)
			assert.Equal(t, tt.expectedExit, exitCode)
		})
	}
}

func TestGetJobOutput(t *testing.T) {
	tests := []struct {
		name           string
		url            string
		handler        http.HandlerFunc
		key            string
		expectedOutput string
		expectedExit   int
	}{
		{
			name: "Success",
			key:  "mykey",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/jobs/test-job", r.URL.Path)
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"id": "test-job", "outputs": {"mykey": "myval"}}`))
			},
			expectedOutput: "myval",
			expectedExit:   0,
		},
		{
			name: "MissingKey",
			key:  "missing",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/jobs/test-job", r.URL.Path)
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"id": "test-job", "outputs": {"mykey": "myval"}}`))
			},
			expectedOutput: "Output key 'missing' not found for job test-job",
			expectedExit:   1,
		},
		{
			name: "ErrorResponse",
			key:  "mykey",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/jobs/test-job", r.URL.Path)
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte(`Job not found`))
			},
			expectedOutput: "Failed to get job info",
			expectedExit:   1,
		},
		{
			name:           "ConnectionError",
			url:            "http://invalid-url",
			key:            "mykey",
			expectedOutput: "Failed to connect to orchestrator",
			expectedExit:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var serverURL string
			if tt.handler != nil {
				server := httptest.NewServer(tt.handler)
				defer server.Close()
				serverURL = server.URL
			} else {
				serverURL = tt.url
			}

			oldStdout := stdout
			var buf bytes.Buffer
			stdout = &buf
			defer func() { stdout = oldStdout }()

			oldExit := exitFunc
			var exitCode int
			exitFunc = func(code int) { exitCode = code }
			defer func() { exitFunc = oldExit }()

			getJobOutput(serverURL, "test-job", tt.key)

			assert.Contains(t, buf.String(), tt.expectedOutput)
			assert.Equal(t, tt.expectedExit, exitCode)
		})
	}
}
