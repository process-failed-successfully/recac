package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAnalyzeReliability_SuccessText(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/analyze/reliability", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "10", r.URL.Query().Get("limit"))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"total_jobs": 10,
			"successful_jobs": 5,
			"flaky_jobs": 3,
			"failed_jobs": 2,
			"success_rate": 80.0,
			"flakiness_rate": 30.0,
			"failure_rate": 20.0,
			"total_retries": 5,
			"top_flaky_jobs": [
				{"summary": "Flaky Task", "occurrences": 3, "total_retries": 5, "avg_retries": 1.66}
			],
			"top_failing_jobs": [
				{"summary": "Fatal Task", "occurrences": 2}
			]
		}`))
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

	analyzeReliability(server.URL, 10, "text")

	assert.Equal(t, 0, exitCode)
	output := out.String()
	assert.Contains(t, output, "Pipeline Reliability Report")
	assert.Contains(t, output, "Total Evaluated Jobs:")
	assert.Contains(t, output, "Flaky Jobs:")
	assert.Contains(t, output, "30.00%")
	assert.Contains(t, output, "Top 10 Flaky Jobs")
	assert.Contains(t, output, "Flaky Task")
	assert.Contains(t, output, "Top 10 Failing Jobs")
	assert.Contains(t, output, "Fatal Task")
}

func TestAnalyzeReliability_SuccessJson(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/analyze/reliability", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "5", r.URL.Query().Get("limit"))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"total_jobs": 10,
			"successful_jobs": 8,
			"flaky_jobs": 2,
			"failed_jobs": 0,
			"success_rate": 100.0,
			"flakiness_rate": 20.0,
			"failure_rate": 0.0,
			"total_retries": 2,
			"top_flaky_jobs": [],
			"top_failing_jobs": []
		}`))
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

	analyzeReliability(server.URL, 5, "json")

	assert.Equal(t, 0, exitCode)
	output := out.String()
	assert.Contains(t, output, `"total_jobs": 10`)
	assert.Contains(t, output, `"success_rate": 100`)
	assert.NotContains(t, output, "Pipeline Reliability Report") // Should not contain text UI
}

func TestAnalyzeReliability_ConnectionError(t *testing.T) {
	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	analyzeReliability("http://invalid-host:12345", 10, "text")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to connect to orchestrator")
}

func TestAnalyzeReliability_ErrorResponse(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/analyze/reliability", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`Internal Server Error`))
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

	analyzeReliability(server.URL, 10, "text")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to fetch reliability stats")
}
