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

func TestInspectDataflow_WithOutputs(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/jobs/JOB-TARGET", func(w http.ResponseWriter, r *http.Request) {
		job := orchestrator.JobInfo{
			ID:     "JOB-TARGET",
			Status: "Pending",
			WorkItem: orchestrator.WorkItem{
				ID:        "JOB-TARGET",
				DependsOn: []string{"DEP-1", "DEP-2"},
			},
		}
		json.NewEncoder(w).Encode(job)
	})

	mux.HandleFunc("/jobs/DEP-1", func(w http.ResponseWriter, r *http.Request) {
		job := orchestrator.JobInfo{
			ID:     "DEP-1",
			Status: "Completed",
			Outputs: map[string]string{
				"URL": "https://example.com",
				"KEY": "12345",
			},
		}
		json.NewEncoder(w).Encode(job)
	})

	mux.HandleFunc("/jobs/DEP-2", func(w http.ResponseWriter, r *http.Request) {
		job := orchestrator.JobInfo{
			ID:     "DEP-2",
			Status: "Completed",
			Outputs: map[string]string{
				"MESSAGE": "Hello World",
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

	inspectDataflow(server.URL, "JOB-TARGET")

	output := buf.String()
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, output, "Dataflow Inspection: JOB-TARGET")
	assert.Contains(t, output, "Upstream Dependencies: DEP-1, DEP-2")
	assert.Contains(t, output, "DEP-1:")
	assert.Contains(t, output, "DEP_DEP_1_URL=https://example.com")
	assert.Contains(t, output, "DEP_DEP_1_KEY=12345")
	assert.Contains(t, output, "DEP-2:")
	assert.Contains(t, output, "DEP_DEP_2_MESSAGE=Hello World")
	assert.Contains(t, output, "These variables will be injected into the job's environment when it runs.")
}

func TestInspectDataflow_NoOutputs(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/jobs/JOB-TARGET", func(w http.ResponseWriter, r *http.Request) {
		job := orchestrator.JobInfo{
			ID:     "JOB-TARGET",
			Status: "Pending",
			WorkItem: orchestrator.WorkItem{
				ID:        "JOB-TARGET",
				DependsOn: []string{"DEP-NO-OUTPUT"},
			},
		}
		json.NewEncoder(w).Encode(job)
	})

	mux.HandleFunc("/jobs/DEP-NO-OUTPUT", func(w http.ResponseWriter, r *http.Request) {
		job := orchestrator.JobInfo{
			ID:     "DEP-NO-OUTPUT",
			Status: "Running",
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

	inspectDataflow(server.URL, "JOB-TARGET")

	output := buf.String()
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, output, "Dataflow Inspection: JOB-TARGET")
	assert.Contains(t, output, "Upstream Dependencies: DEP-NO-OUTPUT")
	assert.Contains(t, output, "(No outputs generated)")
	assert.Contains(t, output, "None of the upstream dependencies have generated outputs yet.")
}

func TestInspectDataflow_NoDependencies(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/jobs/JOB-TARGET", func(w http.ResponseWriter, r *http.Request) {
		job := orchestrator.JobInfo{
			ID:     "JOB-TARGET",
			Status: "Pending",
			WorkItem: orchestrator.WorkItem{
				ID: "JOB-TARGET",
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

	inspectDataflow(server.URL, "JOB-TARGET")

	output := buf.String()
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, output, "Dataflow Inspection: JOB-TARGET")
	assert.Contains(t, output, "This job has no upstream dependencies.")
}

func TestInspectDataflow_ConnectionError(t *testing.T) {
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	exitCalled := false
	oldExit := exitFunc
	exitFunc = func(code int) { exitCalled = true }
	defer func() { exitFunc = oldExit }()

	inspectDataflow("http://localhost:0", "job-1")

	assert.True(t, exitCalled)
	output := buf.String()
	assert.Contains(t, output, "Failed to connect to orchestrator")
}

func TestInspectDataflow_HTTPError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/job-1", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`internal error`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	exitCalled := false
	oldExit := exitFunc
	exitFunc = func(code int) { exitCalled = true }
	defer func() { exitFunc = oldExit }()

	inspectDataflow(server.URL, "job-1")

	assert.True(t, exitCalled)
	output := buf.String()
	assert.Contains(t, output, "Failed to fetch job details: internal error")
}

func TestInspectDataflow_DecodeError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/job-1", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`invalid json`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	exitCalled := false
	oldExit := exitFunc
	exitFunc = func(code int) { exitCalled = true }
	defer func() { exitFunc = oldExit }()

	inspectDataflow(server.URL, "job-1")

	assert.True(t, exitCalled)
	output := buf.String()
	assert.Contains(t, output, "Failed to decode response")
}

func TestInspectDataflow_DepFetchError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/job-1", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"id": "job-1",
			"work_item": {
				"depends_on": ["job-0"]
			}
		}`))
	})
	mux.HandleFunc("/jobs/job-0", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`internal error`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	exitCalled := false
	oldExit := exitFunc
	exitFunc = func(code int) { exitCalled = true }
	defer func() { exitFunc = oldExit }()

	inspectDataflow(server.URL, "job-1")

	assert.False(t, exitCalled)
	output := buf.String()
	assert.Contains(t, output, "Could not fetch dependency (status 500)")
}

func TestInspectDataflow_DepDecodeError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/job-1", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"id": "job-1",
			"work_item": {
				"depends_on": ["job-0"]
			}
		}`))
	})
	mux.HandleFunc("/jobs/job-0", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`invalid json`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	exitCalled := false
	oldExit := exitFunc
	exitFunc = func(code int) { exitCalled = true }
	defer func() { exitFunc = oldExit }()

	inspectDataflow(server.URL, "job-1")

	assert.False(t, exitCalled)
	output := buf.String()
	assert.Contains(t, output, "Failed to decode dependency")
}
