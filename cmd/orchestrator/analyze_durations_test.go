package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAnalyzeDurations(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "completed", r.URL.Query().Get("state"))

		// Return a mix of completed jobs with various durations and tags
		now := time.Now()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[` +
			`{"id": "job-1", "summary": "Quick job", "status": "Completed", "start_time": "` + now.Add(-1*time.Minute).Format(time.RFC3339) + `", "end_time": "` + now.Format(time.RFC3339) + `", "work_item": {"tags": ["fast"]}},` +
			`{"id": "job-2", "summary": "Slow job", "status": "Completed", "start_time": "` + now.Add(-10*time.Minute).Format(time.RFC3339) + `", "end_time": "` + now.Format(time.RFC3339) + `", "work_item": {"tags": ["slow", "backend"]}},` +
			`{"id": "job-3", "summary": "Medium job", "status": "Completed", "start_time": "` + now.Add(-5*time.Minute).Format(time.RFC3339) + `", "end_time": "` + now.Format(time.RFC3339) + `", "work_item": {"tags": ["backend"]}},` +
			`{"id": "job-4", "summary": "Invalid job (no end)", "status": "Running", "start_time": "` + now.Add(-5*time.Minute).Format(time.RFC3339) + `", "work_item": {"tags": ["backend"]}}` +
			`]`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	analyzeDurations(server.URL, 2)

	assert.Equal(t, 0, exitCode)
	output := out.String()

	// Verify headers and content
	assert.Contains(t, output, "Duration Analysis (3 valid jobs)")
	assert.Contains(t, output, "Mean:")
	assert.Contains(t, output, "Median:")
	assert.Contains(t, output, "Min:")
	assert.Contains(t, output, "Max:")

	// Verify tag stats
	assert.Contains(t, output, "Average Duration by Tag")
	assert.Contains(t, output, "fast")
	assert.Contains(t, output, "slow")
	assert.Contains(t, output, "backend")

	// Verify slowest jobs
	assert.Contains(t, output, "Top 2 Slowest Jobs")
	assert.Contains(t, output, "job-2")
	assert.Contains(t, output, "job-3")
	assert.NotContains(t, output, "job-1") // Limit was 2
}

func TestAnalyzeDurations_NoValidJobs(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	analyzeDurations(server.URL, 10)

	assert.Equal(t, 0, exitCode)
	output := out.String()
	assert.Contains(t, output, "No valid completed jobs with duration found.")
}

func TestAnalyzeDurations_ServerError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`internal server error`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	analyzeDurations(server.URL, 10)

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to fetch completed jobs")
	assert.Contains(t, out.String(), "internal server error")
}

func TestAnalyzeDurations_ConnectionError(t *testing.T) {
	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	analyzeDurations("http://invalid-host:12345", 10)

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to connect to orchestrator")
}
