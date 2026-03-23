package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"recac/internal/orchestrator"

	"github.com/stretchr/testify/assert"
)

func TestGetJobOutput(t *testing.T) {
	job := orchestrator.JobInfo{
		ID:      "job1",
		Status:  "Completed",
		Outputs: map[string]string{"key1": "val1", "key2": "val2"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs/job1" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(job)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	var buf bytes.Buffer
	stdout = &buf
	exitFunc = func(int) {} // Mock exit

	getJobOutput(server.URL, "job1", "key1")
	assert.Contains(t, buf.String(), "val1")

	buf.Reset()
	getJobOutput(server.URL, "job1", "")
	assert.Contains(t, buf.String(), "key1")
	assert.Contains(t, buf.String(), "val1")
	assert.Contains(t, buf.String(), "key2")
	assert.Contains(t, buf.String(), "val2")
}

func TestInspectDataFlow(t *testing.T) {
	depJob := orchestrator.JobInfo{
		ID:      "dep1",
		Status:  "Completed",
		Outputs: map[string]string{"out1": "val1"},
	}
	targetJob := orchestrator.JobInfo{
		ID: "target1",
		WorkItem: orchestrator.WorkItem{
			DependsOn: []string{"dep1"},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs/target1" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(targetJob)
		} else if r.URL.Path == "/jobs/dep1" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(depJob)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	var buf bytes.Buffer
	stdout = &buf
	exitFunc = func(int) {}

	inspectDataFlow(server.URL, "target1")
	output := buf.String()
	assert.Contains(t, output, "Data Flow for Job: target1")
	assert.Contains(t, output, "[dep1]")
	assert.Contains(t, output, "out1")
	assert.Contains(t, output, "val1")
	assert.Contains(t, output, "DEP_DEP1_OUT1=val1")
}
