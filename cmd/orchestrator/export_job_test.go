package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"recac/internal/orchestrator"

	"github.com/stretchr/testify/assert"
)

func TestExportSingleJob_SuccessStdout(t *testing.T) {
	mockJob := orchestrator.JobInfo{
		ID:      "TEST-JOB-1",
		Status:  "Completed",
		Summary: "Test job",
		WorkItem: orchestrator.WorkItem{
			ID:          "TEST-JOB-1",
			Summary:     "Test job",
			Description: "Test desc",
			RepoURL:     "http://repo.git",
			Tags:        []string{"tag1", "tag2"},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs/TEST-JOB-1", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)
		json.NewEncoder(w).Encode(mockJob)
	}))
	defer server.Close()

	var buf bytes.Buffer
	stdout = &buf
	defer func() { stdout = os.Stdout }()

	var exitCode int
	exitFunc = func(code int) {
		exitCode = code
	}
	defer func() { exitFunc = os.Exit }()

	exportSingleJob(server.URL, "TEST-JOB-1", "")

	assert.Equal(t, 0, exitCode)

	var exportedWorkItem orchestrator.WorkItem
	err := json.Unmarshal(buf.Bytes(), &exportedWorkItem)
	assert.NoError(t, err)
	assert.Equal(t, mockJob.WorkItem.ID, exportedWorkItem.ID)
	assert.Equal(t, mockJob.WorkItem.Summary, exportedWorkItem.Summary)
	assert.Equal(t, mockJob.WorkItem.Tags, exportedWorkItem.Tags)
}

func TestExportSingleJob_SuccessFile(t *testing.T) {
	mockJob := orchestrator.JobInfo{
		ID:      "TEST-JOB-1",
		Status:  "Completed",
		Summary: "Test job",
		WorkItem: orchestrator.WorkItem{
			ID:          "TEST-JOB-1",
			Summary:     "Test job",
			Description: "Test desc",
			RepoURL:     "http://repo.git",
			Tags:        []string{"tag1", "tag2"},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs/TEST-JOB-1", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)
		json.NewEncoder(w).Encode(mockJob)
	}))
	defer server.Close()

	var buf bytes.Buffer
	stdout = &buf
	defer func() { stdout = os.Stdout }()

	var exitCode int
	exitFunc = func(code int) {
		exitCode = code
	}
	defer func() { exitFunc = os.Exit }()

	tempDir := t.TempDir()
	outFile := filepath.Join(tempDir, "export.json")

	exportSingleJob(server.URL, "TEST-JOB-1", outFile)

	assert.Equal(t, 0, exitCode)
	assert.Contains(t, buf.String(), "exported to "+outFile+" successfully")

	content, err := os.ReadFile(outFile)
	assert.NoError(t, err)

	var exportedWorkItem orchestrator.WorkItem
	err = json.Unmarshal(content, &exportedWorkItem)
	assert.NoError(t, err)
	assert.Equal(t, mockJob.WorkItem.ID, exportedWorkItem.ID)
	assert.Equal(t, mockJob.WorkItem.Summary, exportedWorkItem.Summary)
	assert.Equal(t, mockJob.WorkItem.Tags, exportedWorkItem.Tags)
}

func TestExportSingleJob_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("job not found"))
	}))
	defer server.Close()

	var buf bytes.Buffer
	stdout = &buf
	defer func() { stdout = os.Stdout }()

	var exitCode int
	exitFunc = func(code int) {
		exitCode = code
	}
	defer func() { exitFunc = os.Exit }()

	exportSingleJob(server.URL, "NON-EXISTENT", "")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, buf.String(), "Failed to fetch job NON-EXISTENT: status 404 Not Found - job not found")
}

func TestExportSingleJob_ConnectionError(t *testing.T) {
	var buf bytes.Buffer
	stdout = &buf
	defer func() { stdout = os.Stdout }()

	var exitCode int
	exitFunc = func(code int) {
		exitCode = code
	}
	defer func() { exitFunc = os.Exit }()

	exportSingleJob("http://localhost:0", "TEST-JOB-1", "")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, buf.String(), "Failed to connect to orchestrator")
}

func TestExportSingleJob_DecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`invalid json`))
	}))
	defer server.Close()

	var buf bytes.Buffer
	stdout = &buf
	defer func() { stdout = os.Stdout }()

	var exitCode int
	exitFunc = func(code int) {
		exitCode = code
	}
	defer func() { exitFunc = os.Exit }()

	exportSingleJob(server.URL, "TEST-JOB-1", "")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, buf.String(), "Failed to decode job response")
}

func TestExportSingleJob_FileError(t *testing.T) {
	mockJob := orchestrator.JobInfo{
		ID: "TEST-JOB-1",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(mockJob)
	}))
	defer server.Close()

	var buf bytes.Buffer
	stdout = &buf
	defer func() { stdout = os.Stdout }()

	var exitCode int
	exitFunc = func(code int) {
		exitCode = code
	}
	defer func() { exitFunc = os.Exit }()

	// Provide a directory path instead of a file
	exportSingleJob(server.URL, "TEST-JOB-1", t.TempDir())

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, buf.String(), "Failed to write exported job to file")
}
