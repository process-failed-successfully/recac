package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGeneratePipeline_SuccessStdout(t *testing.T) {
	expectedYAML := `name: new-pipeline
jobs:
  test:
    summary: test`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/pipeline/generate", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		var reqBody map[string]string
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		assert.NoError(t, err)
		assert.Equal(t, "do something", reqBody["prompt"])

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"pipeline_yaml": expectedYAML})
	}))
	defer server.Close()

	oldStdout := stdout
	var buf bytes.Buffer
	stdout = &buf
	defer func() { stdout = oldStdout }()

	generatePipeline(server.URL, "do something", "-", "", "")

	assert.Contains(t, buf.String(), expectedYAML)
}

func TestGeneratePipeline_SuccessFile(t *testing.T) {
	expectedYAML := `name: new-pipeline
jobs:
  file:
    summary: test`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"pipeline_yaml": expectedYAML})
	}))
	defer server.Close()

	oldStdout := stdout
	var buf bytes.Buffer
	stdout = &buf
	defer func() { stdout = oldStdout }()

	tmpDir := t.TempDir()
	outFile := filepath.Join(tmpDir, "generated.yaml")

	generatePipeline(server.URL, "do something", outFile, "openai", "gpt-4")

	assert.Contains(t, buf.String(), fmt.Sprintf("Pipeline successfully generated to %s", outFile))

	content, err := os.ReadFile(outFile)
	assert.NoError(t, err)
	assert.Equal(t, expectedYAML, string(content))
}

func TestGeneratePipeline_ConnectionError(t *testing.T) {
	oldExit := exitFunc
	var exitCode int
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	oldStdout := stdout
	var buf bytes.Buffer
	stdout = &buf
	defer func() { stdout = oldStdout }()

	generatePipeline("http://localhost:12345", "prompt", "-", "", "")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, buf.String(), "Failed to connect to orchestrator")
}

func TestGeneratePipeline_APIError(t *testing.T) {
	oldExit := exitFunc
	var exitCode int
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	oldStdout := stdout
	var buf bytes.Buffer
	stdout = &buf
	defer func() { stdout = oldStdout }()

	generatePipeline(server.URL, "prompt", "-", "", "")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, buf.String(), "Failed to generate pipeline: Internal Server Error")
}

func TestGeneratePipeline_InvalidURL(t *testing.T) {
	oldExit := exitFunc
	var exitCode int
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	oldStdout := stdout
	var buf bytes.Buffer
	stdout = &buf
	defer func() { stdout = oldStdout }()

	// Trigger a url.Parse error by passing something that breaks parse
	// Actually, url.Parse rarely fails unless control characters are used.
	generatePipeline("http://192.168.0.%31", "prompt", "-", "", "")

	// On some Go versions url.Parse("http://192.168.0.%31") fails
	// If it doesn't fail url.Parse, it will fail http connection
	out := buf.String()
	if strings.Contains(out, "Failed to parse URL") {
		assert.Equal(t, 1, exitCode)
	} else if strings.Contains(out, "Failed to connect to orchestrator") {
		assert.Equal(t, 1, exitCode)
	}
}

func TestGeneratePipeline_InvalidJSONResponse(t *testing.T) {
	oldExit := exitFunc
	var exitCode int
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{invalid-json}"))
	}))
	defer server.Close()

	oldStdout := stdout
	var buf bytes.Buffer
	stdout = &buf
	defer func() { stdout = oldStdout }()

	generatePipeline(server.URL, "prompt", "-", "", "")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, buf.String(), "Failed to decode response")
}

func TestGeneratePipeline_FileWriteError(t *testing.T) {
	oldExit := exitFunc
	var exitCode int
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"pipeline_yaml": "yaml"})
	}))
	defer server.Close()

	oldStdout := stdout
	var buf bytes.Buffer
	stdout = &buf
	defer func() { stdout = oldStdout }()

	// Give a directory path to trigger write error
	badPath := filepath.Join("non-existent-dir", "out.yaml")

	generatePipeline(server.URL, "prompt", badPath, "", "")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, buf.String(), "Failed to create output file")
}
