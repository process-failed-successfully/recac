package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAnalyzeCosts(t *testing.T) {
	// Mock the exit function so tests don't exit
	oldExit := exitFunc
	exitFunc = func(code int) {
		panic("exit")
	}
	defer func() { exitFunc = oldExit }()

	// Mock stdout
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	// Mock the orchestrator server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs/analyze/costs", r.URL.Path)
		assert.Equal(t, "10", r.URL.Query().Get("limit"))
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"total_jobs": 3,
			"total_cost": 5.5,
			"total_tokens": 5500.0,
			"cost_by_tag": [
				{"tag": "tag-a", "cost": 5.0, "count": 2},
				{"tag": "tag-b", "cost": 3.5, "count": 1},
				{"tag": "tag-c", "cost": 0.5, "count": 1}
			],
			"cost_by_model": [
				{"model": "model-y", "cost": 3.5, "count": 1},
				{"model": "model-x", "cost": 2.0, "count": 2}
			],
			"top_expensive_jobs": [
				{"id": "JOB-2", "summary": "Summary 2", "status": "Completed", "metrics": {"cost": 3.5, "tokens": 3500}},
				{"id": "JOB-1", "summary": "Summary 1", "status": "Completed", "metrics": {"cost": 1.5, "tokens": 1500}},
				{"id": "JOB-3", "summary": "Summary 3", "status": "Failed", "metrics": {"cost": 0.5, "tokens": 500}}
			]
		}`))
	}))
	defer server.Close()

	// Call the function
	analyzeCosts(server.URL, 10, "text")

	// Verify output
	output := buf.String()
	assert.Contains(t, output, "Pipeline Cost Analysis (3 evaluated jobs)")
	assert.Contains(t, output, "Total Cost:")
	assert.Contains(t, output, "$5.5000")
	assert.Contains(t, output, "Total Tokens:")
	assert.Contains(t, output, "5500")

	assert.Contains(t, output, "Cost by Model")
	assert.Contains(t, output, "model-y")
	assert.Contains(t, output, "$3.5000")
	assert.Contains(t, output, "model-x")
	assert.Contains(t, output, "$2.0000")

	assert.Contains(t, output, "Cost by Tag")
	assert.Contains(t, output, "tag-a")
	assert.Contains(t, output, "$5.0000")
	assert.Contains(t, output, "tag-c")
	assert.Contains(t, output, "$0.5000")

	assert.Contains(t, output, "Top 10 Expensive Jobs")
	assert.Contains(t, output, "JOB-2")
	assert.Contains(t, output, "JOB-1")
	assert.Contains(t, output, "JOB-3")
}

func TestAnalyzeCostsJSON(t *testing.T) {
	// Mock the exit function
	oldExit := exitFunc
	exitFunc = func(code int) {
		panic("exit")
	}
	defer func() { exitFunc = oldExit }()

	// Mock stdout
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	// Mock the orchestrator server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"total_jobs": 3,
			"total_cost": 5.5,
			"total_tokens": 5500.0,
			"cost_by_tag": [
				{"tag": "tag-a", "cost": 5.0, "count": 2},
				{"tag": "tag-b", "cost": 3.5, "count": 1},
				{"tag": "tag-c", "cost": 0.5, "count": 1}
			],
			"cost_by_model": [
				{"model": "model-y", "cost": 3.5, "count": 1},
				{"model": "model-x", "cost": 2.0, "count": 2}
			],
			"top_expensive_jobs": [
				{"id": "JOB-2", "summary": "Summary 2", "status": "Completed", "metrics": {"cost": 3.5, "tokens": 3500}},
				{"id": "JOB-1", "summary": "Summary 1", "status": "Completed", "metrics": {"cost": 1.5, "tokens": 1500}},
				{"id": "JOB-3", "summary": "Summary 3", "status": "Failed", "metrics": {"cost": 0.5, "tokens": 500}}
			]
		}`))
	}))
	defer server.Close()

	// Call the function
	analyzeCosts(server.URL, 10, "json")

	// Verify JSON output
	output := buf.String()
	assert.Contains(t, output, `"total_jobs": 3`)
	assert.Contains(t, output, `"total_cost": 5.5`)
	assert.Contains(t, output, `"total_tokens": 5500`)
	assert.Contains(t, output, `"cost_by_tag"`)
	assert.Contains(t, output, `"cost_by_model"`)
	assert.Contains(t, output, `"top_expensive_jobs"`)
}
