package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"recac/internal/orchestrator"
	"github.com/stretchr/testify/assert"
)

func TestPrintTimeline(t *testing.T) {
	now := time.Now()
	jobs := []orchestrator.JobInfo{
		{
			ID:        "JOB-1",
			Status:    "Completed",
			StartTime: now.Add(-10 * time.Minute),
			EndTime:   now.Add(-8 * time.Minute),
		},
		{
			ID:        "JOB-2",
			Status:    "Failed",
			StartTime: now.Add(-8 * time.Minute),
			EndTime:   now.Add(-5 * time.Minute),
		},
		{
			ID:        "JOB-3",
			Status:    "Running",
			StartTime: now.Add(-5 * time.Minute),
			EndTime:   time.Time{}, // still running
		},
		{
			ID:        "JOB-4-UNSTARTED",
			Status:    "Pending",
			StartTime: time.Time{}, // Should be filtered out
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs", r.URL.Path)
		assert.Equal(t, "all", r.URL.Query().Get("state"))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jobs)
	}))
	defer server.Close()

	// Redirect stdout to capture output
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	// Override exitFunc so we don't accidentally terminate the test runner
	oldExitFunc := exitFunc
	exitFunc = func(code int) {
		t.Fatalf("exitFunc called with code %d", code)
	}
	defer func() { exitFunc = oldExitFunc }()

	// Execute
	printTimeline(server.URL, 20)

	output := buf.String()

	// Verify Output Elements
	assert.Contains(t, output, "Execution Timeline (Total Window: 10m0s)")
	assert.Contains(t, output, "JOB-1")
	assert.Contains(t, output, "JOB-2")
	assert.Contains(t, output, "JOB-3")
	assert.NotContains(t, output, "JOB-4-UNSTARTED")

	// Verify status strings in output
	assert.Contains(t, output, "(Completed)")
	assert.Contains(t, output, "(Failed)")
	assert.Contains(t, output, "(Running)")

	// Ensure bars are rendered (█ character)
	assert.Contains(t, output, "█")
}

func TestPrintTimeline_NoJobs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]orchestrator.JobInfo{})
	}))
	defer server.Close()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	oldExitFunc := exitFunc
	exitFunc = func(code int) {
		t.Fatalf("exitFunc called with code %d", code)
	}
	defer func() { exitFunc = oldExitFunc }()

	printTimeline(server.URL, 20)

	output := buf.String()
	assert.Contains(t, output, "No started jobs to display in timeline.")
}
