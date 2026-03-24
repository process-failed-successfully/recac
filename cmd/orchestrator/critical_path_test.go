package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPrintCriticalPath_Success(t *testing.T) {
	now := time.Now()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs", r.URL.Path)
		assert.Equal(t, "all", r.URL.Query().Get("state"))

		jobsJSON := `[
			{"id": "A", "start_time": "` + now.Format(time.RFC3339Nano) + `", "end_time": "` + now.Add(10*time.Second).Format(time.RFC3339Nano) + `", "status": "Completed", "work_item": {"id": "A", "depends_on": []}},
			{"id": "B", "start_time": "` + now.Add(10*time.Second).Format(time.RFC3339Nano) + `", "end_time": "` + now.Add(30*time.Second).Format(time.RFC3339Nano) + `", "status": "Completed", "work_item": {"id": "B", "depends_on": ["A"]}}
		]`

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(jobsJSON))
	}))
	defer server.Close()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	printCriticalPath(server.URL)

	assert.Equal(t, 0, exitCode)
	out := buf.String()
	assert.Contains(t, out, "Critical Path Analysis (Total Critical Duration: 30s)")
	assert.Contains(t, out, "A [10s] (Completed)")
	assert.Contains(t, out, "B [20s] (Completed)")
	assert.Contains(t, out, "↓")
}

func TestPrintCriticalPath_ConnectionError(t *testing.T) {
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	printCriticalPath("http://invalid-host:12345")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, buf.String(), "Failed to connect to orchestrator")
}

func TestPrintCriticalPath_NoJobs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer server.Close()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	printCriticalPath(server.URL)

	assert.Equal(t, 0, exitCode)
	assert.Contains(t, buf.String(), "No jobs available for critical path analysis")
}

func TestPrintCriticalPath_ErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	printCriticalPath(server.URL)

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, buf.String(), "Failed to fetch jobs:")
}
