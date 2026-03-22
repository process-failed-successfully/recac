package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExportTrace_Stdout(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/export/trace", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "all", r.URL.Query().Get("state"))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"name": "job-1"}]`))
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

	exportTrace(server.URL, "-", "all")

	assert.Equal(t, 0, exitCode)
	assert.Contains(t, out.String(), `[{"name": "job-1"}]`)
}

func TestExportTrace_File(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/export/trace", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"name": "job-1"}]`))
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

	tmpDir, err := os.MkdirTemp("", "trace_test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	outFile := filepath.Join(tmpDir, "trace.json")

	exportTrace(server.URL, outFile, "")

	assert.Equal(t, 0, exitCode)
	assert.Contains(t, out.String(), "Trace successfully exported to")

	content, err := os.ReadFile(outFile)
	assert.NoError(t, err)
	assert.Equal(t, `[{"name": "job-1"}]`, string(content))
}

func TestExportTrace_ConnectionError(t *testing.T) {
	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	exportTrace("http://invalid-host:12345", "-", "")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to connect to orchestrator")
}

func TestExportTrace_ErrorResponse(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/export/trace", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
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

	exportTrace(server.URL, "-", "")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to export trace: Internal Server Error")
}
