package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAnalyzeCosts_SuccessText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs/analyze/costs", r.URL.Path)
		assert.Equal(t, "10", r.URL.Query().Get("limit"))

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"total_stats": {
				"total_cost": 150.75,
				"total_tokens_prompt": 100000,
				"total_tokens_completion": 50000,
				"total_jobs": 10
			},
			"model_stats": [
				{"model": "gpt-4", "cost": 100.0, "jobs_count": 5},
				{"model": "gpt-3.5", "cost": 50.75, "jobs_count": 5}
			],
			"tag_stats": [
				{"tag": "backend", "cost": 80.0, "jobs_count": 4}
			],
			"top_expensive_jobs": [
				{"id": "job-123", "summary": "Expensive Job", "metrics": {"cost_usd": 25.0}}
			]
		}`))
	}))
	defer server.Close()

	oldExit := exitFunc
	var exitCode int
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	oldStdout := stdout
	r, w, _ := os.Pipe()
	stdout = w
	defer func() { stdout = oldStdout }()

	analyzeCosts(server.URL, 10, "text")

	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	assert.Contains(t, output, "AI Cost Analysis")
	assert.Contains(t, output, "Total Cost:")
	assert.Contains(t, output, "$150.7500")
	assert.Contains(t, output, "Total Evaluated Jobs:")
	assert.Contains(t, output, "10")
	assert.Contains(t, output, "Cost by Model")
	assert.Contains(t, output, "gpt-4")
	assert.Contains(t, output, "$100.0000")
	assert.Contains(t, output, "Cost by Tag")
	assert.Contains(t, output, "backend")
	assert.Contains(t, output, "Top 1 Most Expensive Jobs")
	assert.Contains(t, output, "Expensive Job")
	assert.Contains(t, output, "$25.0000")

	assert.Equal(t, 0, exitCode)
}

func TestAnalyzeCosts_SuccessJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs/analyze/costs", r.URL.Path)
		assert.Equal(t, "5", r.URL.Query().Get("limit"))

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"total_stats": {
				"total_cost": 10.0,
				"total_jobs": 1
			},
			"tag_stats": [],
			"model_stats": [],
			"top_expensive_jobs": []
		}`))
	}))
	defer server.Close()

	oldExit := exitFunc
	var exitCode int
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	oldStdout := stdout
	r, w, _ := os.Pipe()
	stdout = w
	defer func() { stdout = oldStdout }()

	analyzeCosts(server.URL, 5, "json")

	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	assert.Contains(t, output, `"total_cost": 10`)
	assert.NotContains(t, output, "AI Cost Analysis")

	assert.Equal(t, 0, exitCode)
}

func TestAnalyzeCosts_NoJobs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"total_stats": {"total_jobs": 0}}`))
	}))
	defer server.Close()

	oldExit := exitFunc
	var exitCode int
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	oldStdout := stdout
	r, w, _ := os.Pipe()
	stdout = w
	defer func() { stdout = oldStdout }()

	analyzeCosts(server.URL, 10, "text")

	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := strings.TrimSpace(buf.String())

	assert.Equal(t, "No valid completed jobs with cost data found.", output)
	assert.Equal(t, 0, exitCode)
}
