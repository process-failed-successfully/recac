package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExportTags_JSON(t *testing.T) {
	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := TagStatsResponse{
			Tags: []TagPerformance{
				{Tag: "backend", TotalJobs: 5, SuccessfulJobs: 4, FailedJobs: 1, SuccessRate: 0.8},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	exportTags(server.URL, "-", "json", 10)

	assert.Equal(t, 0, exitCode)
	if !strings.Contains(out.String(), `"tag": "backend"`) {
		t.Errorf("Expected 'backend' tag in output, got %s", out.String())
	}
}

func TestExportTags_CSV(t *testing.T) {
	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := TagStatsResponse{
			Tags: []TagPerformance{
				{Tag: "frontend", TotalJobs: 10, SuccessfulJobs: 10, FailedJobs: 0, SuccessRate: 1.0},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	exportTags(server.URL, "-", "csv", 10)

	assert.Equal(t, 0, exitCode)
	if !strings.Contains(out.String(), `frontend,10,10,0,1.00`) {
		t.Errorf("Expected 'frontend' csv row in output, got %s", out.String())
	}
}

func TestExportTags_ServerError(t *testing.T) {
	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`server error`))
	}))
	defer server.Close()

	exportTags(server.URL, "-", "json", 10)

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Error from server")
}

func TestExportTags_ConnectionError(t *testing.T) {
	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	exportTags("http://localhost:12345", "-", "json", 10)

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Error fetching tag analysis")
}

func TestExportTags_DecodeError(t *testing.T) {
	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`invalid json`))
	}))
	defer server.Close()

	exportTags(server.URL, "-", "json", 10)

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Error decoding response")
}

func TestExportTags_ToFile(t *testing.T) {
	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := TagStatsResponse{
			Tags: []TagPerformance{
				{Tag: "backend", TotalJobs: 5, SuccessfulJobs: 4, FailedJobs: 1, SuccessRate: 0.8},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	outFile := filepath.Join(tmpDir, "tags.json")

	exportTags(server.URL, outFile, "json", 10)

	assert.Equal(t, 0, exitCode)
	assert.Contains(t, out.String(), "Tag analysis exported to")

	content, err := os.ReadFile(outFile)
	assert.NoError(t, err)
	assert.Contains(t, string(content), `"tag": "backend"`)
}

func TestExportTags_CreateFileError(t *testing.T) {
	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := TagStatsResponse{
			Tags: []TagPerformance{
				{Tag: "backend", TotalJobs: 5, SuccessfulJobs: 4, FailedJobs: 1, SuccessRate: 0.8},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	exportTags(server.URL, "/root/unwritable/tags.json", "json", 10)

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Error creating output file")
}
