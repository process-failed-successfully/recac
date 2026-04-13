package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidatePipeline_Success(t *testing.T) {
	// Setup mock stdout and exit func
	var buf bytes.Buffer
	origStdout := stdout
	stdout = &buf
	defer func() { stdout = origStdout }()

	exitCalled := false
	origExitFunc := exitFunc
	exitFunc = func(code int) { exitCalled = true }
	defer func() { exitFunc = origExitFunc }()

	// Create a temporary valid pipeline yaml
	tmpDir := t.TempDir()
	pipelineFile := filepath.Join(tmpDir, "valid_pipeline.yaml")
	yamlData := `
name: valid-pipeline
jobs:
  job1:
    summary: "test job"
    task: "do something"
    repo_url: "https://example.com"
`
	err := os.WriteFile(pipelineFile, []byte(yamlData), 0644)
	assert.NoError(t, err)

	validatePipeline(pipelineFile, "", nil)

	assert.False(t, exitCalled, "exitFunc should not be called on success")
	assert.Contains(t, buf.String(), "is valid.")
	assert.Contains(t, buf.String(), "Parsed 1 valid job(s).")
}

func TestValidatePipeline_Failure(t *testing.T) {
	// Setup mock stdout and exit func
	var buf bytes.Buffer
	origStdout := stdout
	stdout = &buf
	defer func() { stdout = origStdout }()

	exitCalled := false
	origExitFunc := exitFunc
	exitFunc = func(code int) { exitCalled = true }
	defer func() { exitFunc = origExitFunc }()

	// Create a temporary invalid pipeline yaml (missing required fields)
	tmpDir := t.TempDir()
	pipelineFile := filepath.Join(tmpDir, "invalid_pipeline.yaml")
	yamlData := `
name: invalid-pipeline
jobs:
  job1:
    summary: "test job"
    depends_on: ["non-existent-job"]
`
	err := os.WriteFile(pipelineFile, []byte(yamlData), 0644)
	assert.NoError(t, err)

	validatePipeline(pipelineFile, "", nil)

	assert.True(t, exitCalled, "exitFunc should be called on failure")
	assert.Contains(t, strings.ToLower(buf.String()), "validation failed")
}

func TestValidatePipeline_FileReadError(t *testing.T) {
	var buf bytes.Buffer
	origStdout := stdout
	stdout = &buf
	defer func() { stdout = origStdout }()

	exitCalled := false
	origExitFunc := exitFunc
	exitFunc = func(code int) { exitCalled = true }
	defer func() { exitFunc = origExitFunc }()

	validatePipeline("nonexistent_file.yaml", "", nil)

	assert.True(t, exitCalled)
	assert.Contains(t, buf.String(), "Failed to read file")
}
