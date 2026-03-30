package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSearchLogs_MatchFound(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs/search/logs", r.URL.Path)
		assert.Equal(t, "panic", r.URL.Query().Get("q"))
		assert.Equal(t, "tag1", r.URL.Query().Get("tag"))
		assert.Equal(t, "failed", r.URL.Query().Get("status"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[
			{
				"job_id": "JOB-123",
				"summary": "Build Backend",
				"status": "Failed",
				"matches": [
					{
						"line_number": 42,
						"text": "panic: runtime error: index out of range",
						"context_before": [{"line_number": 41, "text": "about to panic"}],
						"context_after": [{"line_number": 43, "text": "never reached"}]
					}
				]
			}
		]`))
	}))
	defer server.Close()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() { stdout = oldStdout }()

	searchLogs(server.URL, "panic", "tag1", "failed", 1)

	pw.Close()
	out, _ := io.ReadAll(pr)

	outStr := string(out)
	assert.Contains(t, outStr, "Log Search Results (query: \"panic\")")
	assert.Contains(t, outStr, "Job: JOB-123 (Failed)")
	assert.Contains(t, outStr, "Summary: Build Backend")
	assert.Contains(t, outStr, "Line 41: about to panic")
	assert.Contains(t, outStr, "Line 42: panic: runtime error: index out of range")
	assert.Contains(t, outStr, "Line 43: never reached")
	assert.Equal(t, 0, exitCode)
}

func TestSearchLogs_NoMatch(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer server.Close()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() { stdout = oldStdout }()

	searchLogs(server.URL, "panic", "", "", 0)

	pw.Close()
	out, _ := io.ReadAll(pr)

	outStr := strings.TrimSpace(string(out))
	assert.Equal(t, "No matching logs found.", outStr)
	assert.Equal(t, 0, exitCode)
}

func TestSearchLogs_ConnectionError(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() { stdout = oldStdout }()

	searchLogs("http://localhost:123456", "panic", "", "", 0)

	pw.Close()
	out, _ := io.ReadAll(pr)

	outStr := string(out)
	assert.Contains(t, outStr, "Failed to connect to orchestrator")
	assert.Equal(t, 1, exitCode)
}

func TestSearchLogs_ErrorResponse(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid regex query"))
	}))
	defer server.Close()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() { stdout = oldStdout }()

	searchLogs(server.URL, "[invalid", "", "", 0)

	pw.Close()
	out, _ := io.ReadAll(pr)

	outStr := string(out)
	assert.Contains(t, outStr, "Failed to search logs: invalid regex query")
	assert.Equal(t, 1, exitCode)
}
