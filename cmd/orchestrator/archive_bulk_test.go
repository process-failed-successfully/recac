package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"

	"testing"

	"github.com/stretchr/testify/require"
)

func TestArchiveBulkJobs(t *testing.T) {
	// Setup mock server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/jobs/archive/bulk", r.URL.Path)
		if r.URL.Query().Get("tag") == "test-tag" || r.URL.Query().Get("match") == "test-match" || r.URL.Query().Get("status") == "Failed" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("dummy archive content"))
		} else {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("bad request"))
		}
	}))
	defer ts.Close()

	// Capture stdout
	oldStdout := stdout
	var buf bytes.Buffer
	stdout = &buf
	defer func() { stdout = oldStdout }()

	// Mock exit
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) {
		exitCode = code
	}
	defer func() { exitFunc = oldExit }()

	t.Run("Valid Tag", func(t *testing.T) {
		buf.Reset()
		exitCode = 0
		outPath := filepath.Join(t.TempDir(), "test_tag_archive.tar.gz")

		archiveBulkJobs(ts.URL, "test-tag", "", "", outPath)

		require.Equal(t, 0, exitCode)
		require.Contains(t, buf.String(), "Successfully saved bulk archive")

		content, err := os.ReadFile(outPath)
		require.NoError(t, err)
		require.Equal(t, "dummy archive content", string(content))
	})

	t.Run("Valid Match", func(t *testing.T) {
		buf.Reset()
		exitCode = 0
		outPath := filepath.Join(t.TempDir(), "test_match_archive.tar.gz")

		archiveBulkJobs(ts.URL, "", "test-match", "", outPath)

		require.Equal(t, 0, exitCode)
		require.Contains(t, buf.String(), "Successfully saved bulk archive")

		content, err := os.ReadFile(outPath)
		require.NoError(t, err)
		require.Equal(t, "dummy archive content", string(content))
	})

	t.Run("Valid Status", func(t *testing.T) {
		buf.Reset()
		exitCode = 0
		outPath := filepath.Join(t.TempDir(), "test_status_archive.tar.gz")

		archiveBulkJobs(ts.URL, "", "", "Failed", outPath)

		require.Equal(t, 0, exitCode)
		require.Contains(t, buf.String(), "Successfully saved bulk archive")

		content, err := os.ReadFile(outPath)
		require.NoError(t, err)
		require.Equal(t, "dummy archive content", string(content))
	})

	t.Run("Failed Request", func(t *testing.T) {
		buf.Reset()
		exitCode = 0

		archiveBulkJobs(ts.URL, "invalid-tag", "", "", "")

		require.Equal(t, 1, exitCode)
		require.Contains(t, buf.String(), "Failed to bulk archive jobs: bad request")
	})
}
