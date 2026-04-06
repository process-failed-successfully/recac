package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExportFailures(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "all", r.URL.Query().Get("state"))
		assert.Equal(t, "Failed", r.URL.Query().Get("status"))

		// Return a mix of failed jobs
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[
			{"id": "job-1", "summary": "timeout exceeded", "status": "Failed"},
			{"id": "job-2", "summary": "timeout exceeded", "status": "Failed"},
			{"id": "job-3", "summary": "timeout exceeded", "status": "Failed"},
			{"id": "job-4", "summary": "connection refused", "status": "Failed"},
			{"id": "job-5", "summary": "connection refused", "status": "Failed"},
			{"id": "job-6", "summary": "out of memory", "status": "Failed"},
			{"id": "job-7", "summary": "", "status": "Failed"}
		]`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	t.Run("JSON to stdout", func(t *testing.T) {
		var out bytes.Buffer
		oldStdout := stdout
		stdout = &out
		defer func() { stdout = oldStdout }()

		oldExit := exitFunc
		exitCode := 0
		exitFunc = func(code int) { exitCode = code }
		defer func() { exitFunc = oldExit }()

		exportFailures(server.URL, "-", "json")

		assert.Equal(t, 0, exitCode)

		var stats FailuresExportResponse
		err := json.Unmarshal(out.Bytes(), &stats)
		assert.NoError(t, err)

		assert.Equal(t, 7, stats.TotalFailures)
		assert.Len(t, stats.Failures, 4)

		assert.Equal(t, "timeout exceeded", stats.Failures[0].Summary)
		assert.Equal(t, 3, stats.Failures[0].Occurrences)
		assert.Equal(t, []string{"job-1", "job-2", "job-3"}, stats.Failures[0].JobIDs)

		assert.Equal(t, "connection refused", stats.Failures[1].Summary)
		assert.Equal(t, 2, stats.Failures[1].Occurrences)

		assert.Equal(t, "<empty summary>", stats.Failures[2].Summary)
		assert.Equal(t, 1, stats.Failures[2].Occurrences)

		assert.Equal(t, "out of memory", stats.Failures[3].Summary)
		assert.Equal(t, 1, stats.Failures[3].Occurrences)
	})

	t.Run("CSV to stdout", func(t *testing.T) {
		var out bytes.Buffer
		oldStdout := stdout
		stdout = &out
		defer func() { stdout = oldStdout }()

		oldExit := exitFunc
		exitCode := 0
		exitFunc = func(code int) { exitCode = code }
		defer func() { exitFunc = oldExit }()

		exportFailures(server.URL, "-", "csv")

		assert.Equal(t, 0, exitCode)
		output := out.String()

		assert.Contains(t, output, "Summary,Occurrences,Job IDs")
		assert.Contains(t, output, "timeout exceeded,3,\"job-1, job-2, job-3\"")
		assert.Contains(t, output, "connection refused,2,\"job-4, job-5\"")
		assert.Contains(t, output, "<empty summary>,1,job-7")
		assert.Contains(t, output, "out of memory,1,job-6")
	})

	t.Run("JSON to file", func(t *testing.T) {
		var out bytes.Buffer
		oldStdout := stdout
		stdout = &out
		defer func() { stdout = oldStdout }()

		oldExit := exitFunc
		exitCode := 0
		exitFunc = func(code int) { exitCode = code }
		defer func() { exitFunc = oldExit }()

		tmpFile := filepath.Join(t.TempDir(), "failures.json")
		exportFailures(server.URL, tmpFile, "json")

		assert.Equal(t, 0, exitCode)
		assert.Contains(t, out.String(), "Successfully exported failures analysis")

		data, err := os.ReadFile(tmpFile)
		assert.NoError(t, err)

		var stats FailuresExportResponse
		err = json.Unmarshal(data, &stats)
		assert.NoError(t, err)
		assert.Equal(t, 7, stats.TotalFailures)
	})

	t.Run("CSV to file", func(t *testing.T) {
		var out bytes.Buffer
		oldStdout := stdout
		stdout = &out
		defer func() { stdout = oldStdout }()

		oldExit := exitFunc
		exitCode := 0
		exitFunc = func(code int) { exitCode = code }
		defer func() { exitFunc = oldExit }()

		tmpFile := filepath.Join(t.TempDir(), "failures.csv")
		exportFailures(server.URL, tmpFile, "csv")

		assert.Equal(t, 0, exitCode)
		assert.Contains(t, out.String(), "Successfully exported failures analysis")

		data, err := os.ReadFile(tmpFile)
		assert.NoError(t, err)
		assert.Contains(t, string(data), "Summary,Occurrences,Job IDs")
		assert.Contains(t, string(data), "timeout exceeded,3,\"job-1, job-2, job-3\"")
	})

	t.Run("Server Error", func(t *testing.T) {
		serverErr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("internal error"))
		}))
		defer serverErr.Close()

		var out bytes.Buffer
		oldStdout := stdout
		stdout = &out
		defer func() { stdout = oldStdout }()

		oldExit := exitFunc
		exitCode := 0
		exitFunc = func(code int) { exitCode = code }
		defer func() { exitFunc = oldExit }()

		exportFailures(serverErr.URL, "-", "json")

		assert.Equal(t, 1, exitCode)
		assert.Contains(t, out.String(), "Failed to fetch failed jobs")
	})

	t.Run("Connection Error", func(t *testing.T) {
		var out bytes.Buffer
		oldStdout := stdout
		stdout = &out
		defer func() { stdout = oldStdout }()

		oldExit := exitFunc
		exitCode := 0
		exitFunc = func(code int) { exitCode = code }
		defer func() { exitFunc = oldExit }()

		exportFailures("http://invalid-host:12345", "-", "json")

		assert.Equal(t, 1, exitCode)
		assert.Contains(t, out.String(), "Failed to connect to orchestrator")
	})

	t.Run("Invalid File Path", func(t *testing.T) {
		var out bytes.Buffer
		oldStdout := stdout
		stdout = &out
		defer func() { stdout = oldStdout }()

		oldExit := exitFunc
		exitCode := 0
		exitFunc = func(code int) { exitCode = code }
		defer func() { exitFunc = oldExit }()

		exportFailures(server.URL, "/root/invalid/path/file.json", "json")

		assert.Equal(t, 1, exitCode)
		assert.Contains(t, out.String(), "Failed to create output file")
	})
}
