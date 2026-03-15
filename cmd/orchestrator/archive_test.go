package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArchiveJob_CLI(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs/JOB-123/archive" {
			w.Header().Set("Content-Type", "application/gzip")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("mock-tar-gz-content"))
		} else {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("not found"))
		}
	}))
	defer mockServer.Close()

	t.Run("Success with custom outPath", func(t *testing.T) {
		tmpDir := t.TempDir()
		outPath := filepath.Join(tmpDir, "custom.tar.gz")

		archiveJob(mockServer.URL, "JOB-123", outPath)

		// Check file exists and content
		content, err := os.ReadFile(outPath)
		require.NoError(t, err)
		assert.Equal(t, "mock-tar-gz-content", string(content))
		assert.Equal(t, 0, exitCode)
	})

	t.Run("Success with default outPath", func(t *testing.T) {
		// Change directory to temp dir so default output saves there
		tmpDir := t.TempDir()
		cwd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(cwd)

		archiveJob(mockServer.URL, "JOB-123", "")

		expectedPath := "JOB-123.tar.gz"
		content, err := os.ReadFile(expectedPath)
		require.NoError(t, err)
		assert.Equal(t, "mock-tar-gz-content", string(content))
		assert.Equal(t, 0, exitCode)
	})

	t.Run("API Error", func(t *testing.T) {
		archiveJob(mockServer.URL, "JOB-NOT-FOUND", "")
		assert.Equal(t, 1, exitCode)
	})

	pw.Close()
	io.ReadAll(pr) // clear stdout buffer
}

func TestArchiveBulkJobs_CLI(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs/archive/bulk" && (r.URL.Query().Get("tag") == "mytag" || r.URL.Query().Get("match") == "mymatch") {
			w.Header().Set("Content-Type", "application/gzip")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("mock-bulk-tar-gz-content"))
		} else {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("not found"))
		}
	}))
	defer mockServer.Close()

	t.Run("Success with custom outPath and tag", func(t *testing.T) {
		tmpDir := t.TempDir()
		outPath := filepath.Join(tmpDir, "custom-bulk.tar.gz")

		archiveBulkJobs(mockServer.URL, "mytag", "", "", outPath)

		// Check file exists and content
		content, err := os.ReadFile(outPath)
		require.NoError(t, err)
		assert.Equal(t, "mock-bulk-tar-gz-content", string(content))
		assert.Equal(t, 0, exitCode)
	})

	t.Run("Success with default outPath and match", func(t *testing.T) {
		// Instead of changing directory, just check the file gets created in current dir
		// and clean it up afterward.
		expectedPath := "bulk_archive.tar.gz"
		defer os.Remove(expectedPath)

		archiveBulkJobs(mockServer.URL, "", "mymatch", "", "")

		content, err := os.ReadFile(expectedPath)
		require.NoError(t, err)
		assert.Equal(t, "mock-bulk-tar-gz-content", string(content))
		assert.Equal(t, 0, exitCode)
	})

	t.Run("API Error", func(t *testing.T) {
		archiveBulkJobs(mockServer.URL, "not-found", "", "", "")
		assert.Equal(t, 1, exitCode)
	})

	pw.Close()
	io.ReadAll(pr) // clear stdout buffer
}
