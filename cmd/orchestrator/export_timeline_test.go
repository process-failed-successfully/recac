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

func TestExportTimeline(t *testing.T) {
	originalExit := exitFunc
	defer func() { exitFunc = originalExit }()

	var exitCode int
	exitFunc = func(code int) {
		exitCode = code
	}

	originalStdout := stdout
	var buf bytes.Buffer
	stdout = &buf
	defer func() { stdout = originalStdout }()

	t.Run("ConnectionError", func(t *testing.T) {
		exitCode = 0
		buf.Reset()
		exportTimeline("http://invalid-host", "out.txt", "")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to connect to orchestrator")
	})

	t.Run("BadStatusCode", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		exportTimeline(server.URL, "out.txt", "")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to export timeline: Internal Server Error")
	})

	t.Run("Stdout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/jobs/export/timeline", r.URL.Path)
			assert.Equal(t, "completed", r.URL.Query().Get("state"))
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`gantt
    title Job Execution Timeline
    dateFormat YYYY-MM-DDTHH:mm:ssZ
    axisFormat %H:%M:%S`))
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		exportTimeline(server.URL, "-", "completed")
		assert.Equal(t, 0, exitCode)
		assert.Contains(t, buf.String(), "gantt")
		assert.Contains(t, buf.String(), "title Job Execution Timeline")
	})

	t.Run("FileSuccess", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/jobs/export/timeline", r.URL.Path)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`gantt
    title Job Execution Timeline`))
		}))
		defer server.Close()

		tmpDir := t.TempDir()
		outFile := filepath.Join(tmpDir, "timeline.txt")

		exitCode = 0
		buf.Reset()
		exportTimeline(server.URL, outFile, "")
		assert.Equal(t, 0, exitCode)
		assert.Contains(t, buf.String(), "Timeline successfully exported")

		content, err := os.ReadFile(outFile)
		assert.NoError(t, err)
		assert.Contains(t, string(content), "gantt")
		assert.Contains(t, string(content), "title Job Execution Timeline")
	})

	t.Run("FileCreationError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`gantt`))
		}))
		defer server.Close()

		badPath := filepath.Join("non-existent-dir", "timeline.txt")

		exitCode = 0
		buf.Reset()
		exportTimeline(server.URL, badPath, "")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to create output file")
	})
}
