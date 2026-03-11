package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestImportJobs_Success(t *testing.T) {
	// Mock server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs/batch", r.URL.Path)
		assert.Equal(t, "POST", r.Method)

		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"submitted": ["job-1"]}`)
	}))
	defer ts.Close()

	// Capture output
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	// Temp file for import
	tmpDir := t.TempDir()
	inFile := filepath.Join(tmpDir, "import.json")

	err := os.WriteFile(inFile, []byte(`[{"work_item": {"id": "job-1", "summary": "A job"}}]`), 0644)
	assert.NoError(t, err)

	// Call importJobs
	importJobs(ts.URL, inFile)

	// Assertions
	assert.Contains(t, buf.String(), "Successfully imported jobs:")
	assert.Contains(t, buf.String(), `"submitted": ["job-1"]`)
}

func TestImportJobs_InvalidFile(t *testing.T) {
	// Capture exit
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	// Capture output
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	// Call importJobs with non-existent file
	importJobs("http://localhost:2112", "non_existent_file.json")

	// Assertions
	assert.Equal(t, 1, exitCode)
	assert.Contains(t, buf.String(), "Failed to open file")
}

func TestImportJobs_InvalidJSON(t *testing.T) {
	// Capture exit
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	// Capture output
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	// Temp file for import
	tmpDir := t.TempDir()
	inFile := filepath.Join(tmpDir, "import_invalid.json")

	err := os.WriteFile(inFile, []byte(`not json`), 0644)
	assert.NoError(t, err)

	// Call importJobs
	importJobs("http://localhost:2112", inFile)

	// Assertions
	assert.Equal(t, 1, exitCode)
	assert.Contains(t, buf.String(), "Invalid JSON in file")
}

func TestImportJobs_EmptyFile(t *testing.T) {
	// Capture output
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	// Temp file for import
	tmpDir := t.TempDir()
	inFile := filepath.Join(tmpDir, "import_empty.json")

	err := os.WriteFile(inFile, []byte(`[]`), 0644)
	assert.NoError(t, err)

	// Call importJobs
	importJobs("http://localhost:2112", inFile)

	// Assertions
	assert.Contains(t, buf.String(), "No jobs found in file")
}

func TestImportJobs_ServerError(t *testing.T) {
	// Capture exit
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	// Mock server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, "internal server error")
	}))
	defer ts.Close()

	// Capture output
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	// Temp file for import
	tmpDir := t.TempDir()
	inFile := filepath.Join(tmpDir, "import_err.json")

	err := os.WriteFile(inFile, []byte(`[{"work_item": {"id": "job-1"}}]`), 0644)
	assert.NoError(t, err)

	// Call importJobs
	importJobs(ts.URL, inFile)

	// Assertions
	assert.Equal(t, 1, exitCode)
	assert.Contains(t, buf.String(), "Failed to import jobs: internal server error")
}
