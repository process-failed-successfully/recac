package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"recac/internal/orchestrator"
)

func TestInspectJob_WithBlockers(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/jobs/JOB-BLOCKED", func(w http.ResponseWriter, r *http.Request) {
		job := orchestrator.JobInfo{
			ID:     "JOB-BLOCKED",
			Status: "Pending",
			WorkItem: orchestrator.WorkItem{
				ID:        "JOB-BLOCKED",
				DependsOn: []string{"DEP-1", "DEP-2"},
			},
		}
		json.NewEncoder(w).Encode(job)
	})

	mux.HandleFunc("/jobs/JOB-BLOCKED/blockers", func(w http.ResponseWriter, r *http.Request) {
		blockers := []orchestrator.JobInfo{
			{ID: "DEP-1", Status: "Running"},
			{ID: "DEP-2", Status: "Failed"},
		}
		json.NewEncoder(w).Encode(blockers)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	inspectJob(server.URL, "JOB-BLOCKED")

	output := buf.String()
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, output, "Job Details: JOB-BLOCKED")
	assert.Contains(t, output, "Blockers:")
	assert.Contains(t, output, "- DEP-1")
	assert.Contains(t, output, "(Running)")
	assert.Contains(t, output, "- DEP-2")
	assert.Contains(t, output, "(Failed)")
}

func TestInspectJob_NoBlockers(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/jobs/JOB-NOT-BLOCKED", func(w http.ResponseWriter, r *http.Request) {
		job := orchestrator.JobInfo{
			ID:     "JOB-NOT-BLOCKED",
			Status: "Running", // Status not Pending/Pending Approval, shouldn't call blockers
			WorkItem: orchestrator.WorkItem{
				ID:        "JOB-NOT-BLOCKED",
				DependsOn: []string{"DEP-1"},
			},
		}
		json.NewEncoder(w).Encode(job)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	inspectJob(server.URL, "JOB-NOT-BLOCKED")

	output := buf.String()
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, output, "Job Details: JOB-NOT-BLOCKED")
	assert.NotContains(t, output, "Blockers:")
}
