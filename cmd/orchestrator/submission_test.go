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
	submitAdHocJob(server.URL, "http://repo.com", "My Task", "MY-ID", 0, 0, 0, false, envVars, submitDeps, nil, "group-1", true, "custom-provider", "custom-model")

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

	submitAdHocJob(server.URL, "http://repo.com", "My Task", "", 0, 0, 0, false, nil, nil, nil, "", false, "", "")

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
	t.Setenv("EDITOR", `sh -c 'sed -e "s/\"summary\": \"Original Summary\"/\"summary\": \"Edited Summary\"/g" "$1" > "$1.tmp" && mv "$1.tmp" "$1"' _`)

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
	t.Setenv("EDITOR", `sh -c 'sed -e "s/\"id\": \"MY-JOB\"/\"id\": \"OTHER-JOB\"/g" "$1" > "$1.tmp" && mv "$1.tmp" "$1"' _`)

	editJob(server.URL, "MY-JOB")

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Error: You cannot change the Job ID during edit.")
	assert.Equal(t, 1, exitCode)
}
