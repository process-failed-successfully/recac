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

func TestExportJobs_JSON(t *testing.T) {
	// Mock server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs/export", r.URL.Path)
		assert.Equal(t, "json", r.URL.Query().Get("format"))

		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `[{"id":"job-1","summary":"A job"}]`)
	}))
	defer ts.Close()

	// Capture output
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	// Temp file for export
	tmpDir := t.TempDir()
	outFile := filepath.Join(tmpDir, "export.json")

	// Call exportJobs
	exportJobs(ts.URL, outFile, "json")

	// Assertions
	assert.Contains(t, buf.String(), "Jobs successfully exported")
	assert.FileExists(t, outFile)

	content, err := os.ReadFile(outFile)
	assert.NoError(t, err)
	assert.Contains(t, string(content), "job-1")
}

func TestExportJobs_Stdout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `[{"id":"job-2"}]`)
	}))
	defer ts.Close()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	exportJobs(ts.URL, "-", "json")

	assert.Contains(t, buf.String(), "job-2")
	assert.NotContains(t, buf.String(), "successfully exported")
}

func TestExportJobs_InvalidFormat(t *testing.T) {
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	// Capture exit
	exited := false
	oldExit := exitFunc
	exitFunc = func(code int) { exited = true }
	defer func() { exitFunc = oldExit }()

	exportJobs("http://localhost:2112", "out.txt", "xml")

	assert.True(t, exited)
	assert.Contains(t, buf.String(), "Invalid format")
}
