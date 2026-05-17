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

func TestExportJunit(t *testing.T) {
	// Setup mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs/export", r.URL.Path)
		assert.Equal(t, "junit", r.URL.Query().Get("format"))

		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<testsuites></testsuites>"))
	}))
	defer server.Close()

	// Capture stdout
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	// Avoid os.Exit
	exitCalled := false
	oldExit := exitFunc
	exitFunc = func(code int) { exitCalled = true }
	defer func() { exitFunc = oldExit }()

	t.Run("ExportToStdout", func(t *testing.T) {
		buf.Reset()
		exitCalled = false

		exportJunit(server.URL, "-")

		assert.False(t, exitCalled)
		assert.Contains(t, buf.String(), "<testsuites></testsuites>")
		assert.NotContains(t, buf.String(), "JUnit XML successfully exported to")
	})

	t.Run("ExportToFile", func(t *testing.T) {
		buf.Reset()
		exitCalled = false

		tmpDir := t.TempDir()
		outFile := filepath.Join(tmpDir, "report.xml")

		exportJunit(server.URL, outFile)

		assert.False(t, exitCalled)
		assert.Contains(t, buf.String(), "JUnit XML successfully exported to")

		// Verify file contents
		content, err := os.ReadFile(outFile)
		require.NoError(t, err)
		assert.Equal(t, "<testsuites></testsuites>", string(content))
	})
}

func TestExportJunit_ConnectionError(t *testing.T) {
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	exitCalled := false
	oldExit := exitFunc
	exitFunc = func(code int) { exitCalled = true }
	defer func() { exitFunc = oldExit }()

	exportJunit("http://localhost:0", "-")

	assert.True(t, exitCalled)
	assert.Contains(t, buf.String(), "Failed to connect to orchestrator")
}

func TestExportJunit_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	exitCalled := false
	oldExit := exitFunc
	exitFunc = func(code int) { exitCalled = true }
	defer func() { exitFunc = oldExit }()

	exportJunit(server.URL, "-")

	assert.True(t, exitCalled)
	assert.Contains(t, buf.String(), "Failed to export JUnit XML: status 500")
	assert.Contains(t, buf.String(), "Internal Server Error")
}

func TestExportJunit_FileCreationError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	exitCalled := false
	oldExit := exitFunc
	exitFunc = func(code int) { exitCalled = true }
	defer func() { exitFunc = oldExit }()

	// Use a directory path instead of a file path to force a creation error
	exportJunit(server.URL, t.TempDir())

	assert.True(t, exitCalled)
	assert.Contains(t, buf.String(), "Failed to create output file")
}
