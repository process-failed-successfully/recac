package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExportPipeline_Stdout(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/export/pipeline", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "exported-pipeline", r.URL.Query().Get("name"))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("name: exported-pipeline\njobs: {}"))
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

	exportPipeline(server.URL, "-")

	assert.Equal(t, 0, exitCode)
	assert.Equal(t, "name: exported-pipeline\njobs: {}", out.String())
}

func TestExportPipeline_File(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/export/pipeline", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("name: exported-pipeline\njobs: {}"))
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

	tempDir := t.TempDir()
	exportPath := filepath.Join(tempDir, "out.yaml")

	exportPipeline(server.URL, exportPath)

	assert.Equal(t, 0, exitCode)
	assert.Contains(t, out.String(), "Pipeline successfully exported to")

	content, err := os.ReadFile(exportPath)
	require.NoError(t, err)
	assert.Equal(t, "name: exported-pipeline\njobs: {}", string(content))
}

func TestExportPipeline_Error(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/export/pipeline", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("something went wrong"))
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

	exportPipeline(server.URL, "-")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to export pipeline: something went wrong")
}
