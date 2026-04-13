package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExportPipelineGraphJob_Mermaid(t *testing.T) {
	// Setup mock stdout and exitFunc
	var buf bytes.Buffer
	stdout = &buf
	defer func() { stdout = os.Stdout }()

	exitCalled := false
	exitFunc = func(code int) {
		exitCalled = true
	}
	defer func() { exitFunc = os.Exit }()

	// Create a temporary pipeline file
	tmpDir := t.TempDir()
	pipelineFile := filepath.Join(tmpDir, "pipeline.yaml")
	pipelineContent := `
name: test-pipeline
jobs:
  job1:
    summary: Job 1
  job2:
    summary: Job 2
    depends_on:
      - job1
`
	err := os.WriteFile(pipelineFile, []byte(pipelineContent), 0644)
	assert.NoError(t, err)

	exportPipelineGraphJob(pipelineFile, "", nil, "mermaid", "-")

	assert.False(t, exitCalled)
	output := buf.String()
	assert.Contains(t, output, "graph TD;")
	assert.Contains(t, output, "test-pipeline-job1")
	assert.Contains(t, output, "test-pipeline-job2")
}

func TestExportPipelineGraphJob_FileOut(t *testing.T) {
	// Setup mock stdout and exitFunc
	var buf bytes.Buffer
	stdout = &buf
	defer func() { stdout = os.Stdout }()

	exitCalled := false
	exitFunc = func(code int) {
		exitCalled = true
	}
	defer func() { exitFunc = os.Exit }()

	// Create a temporary pipeline file
	tmpDir := t.TempDir()
	pipelineFile := filepath.Join(tmpDir, "pipeline.yaml")
	outFile := filepath.Join(tmpDir, "out.dot")
	pipelineContent := `
name: test-pipeline
jobs:
  job1:
    summary: Job 1
`
	err := os.WriteFile(pipelineFile, []byte(pipelineContent), 0644)
	assert.NoError(t, err)

	exportPipelineGraphJob(pipelineFile, "", nil, "dot", outFile)

	assert.False(t, exitCalled)
	assert.Contains(t, buf.String(), "Graph exported successfully to")

	// Verify file was written
	outBytes, err := os.ReadFile(outFile)
	assert.NoError(t, err)
	assert.Contains(t, string(outBytes), "digraph G {")
	assert.Contains(t, string(outBytes), "job1")
}

func TestExportPipelineGraphJob_Error(t *testing.T) {
	// Setup mock stdout and exitFunc
	var buf bytes.Buffer
	stdout = &buf
	defer func() { stdout = os.Stdout }()

	exitCalled := false
	exitFunc = func(code int) {
		exitCalled = true
	}
	defer func() { exitFunc = os.Exit }()

	exportPipelineGraphJob("nonexistent_file.yaml", "", nil, "mermaid", "-")

	assert.True(t, exitCalled)
	assert.Contains(t, buf.String(), "Failed to read file")
}
func TestExportPipelineGraphJob_InvalidPipeline(t *testing.T) {
	var buf bytes.Buffer
	stdout = &buf
	defer func() { stdout = os.Stdout }()

	exitCalled := false
	exitFunc = func(code int) {
		exitCalled = true
	}
	defer func() { exitFunc = os.Exit }()

	tmpDir := t.TempDir()
	pipelineFile := filepath.Join(tmpDir, "pipeline.yaml")
	pipelineContent := `
name: test-pipeline
jobs:
  job1:
    summary: Job 1
    depends_on:
      - nonexistent-job
`
	err := os.WriteFile(pipelineFile, []byte(pipelineContent), 0644)
	assert.NoError(t, err)

	exportPipelineGraphJob(pipelineFile, "", nil, "mermaid", "-")

	assert.True(t, exitCalled)
	assert.Contains(t, buf.String(), "Pipeline validation failed")
}

func TestExportPipelineGraphJob_PlantUML(t *testing.T) {
	var buf bytes.Buffer
	stdout = &buf
	defer func() { stdout = os.Stdout }()

	exitCalled := false
	exitFunc = func(code int) {
		exitCalled = true
	}
	defer func() { exitFunc = os.Exit }()

	tmpDir := t.TempDir()
	pipelineFile := filepath.Join(tmpDir, "pipeline.yaml")
	pipelineContent := `
name: test-pipeline
jobs:
  job1:
    summary: Job 1
`
	err := os.WriteFile(pipelineFile, []byte(pipelineContent), 0644)
	assert.NoError(t, err)

	exportPipelineGraphJob(pipelineFile, "", nil, "plantuml", "-")

	assert.False(t, exitCalled)
	output := buf.String()
	assert.Contains(t, output, "@startuml")
	assert.Contains(t, output, "test-pipeline-job1")
}

func TestExportPipelineGraphJob_UnsupportedFormat(t *testing.T) {
	var buf bytes.Buffer
	stdout = &buf
	defer func() { stdout = os.Stdout }()

	exitCalled := false
	exitFunc = func(code int) {
		exitCalled = true
	}
	defer func() { exitFunc = os.Exit }()

	tmpDir := t.TempDir()
	pipelineFile := filepath.Join(tmpDir, "pipeline.yaml")
	pipelineContent := `
name: test-pipeline
jobs:
  job1:
    summary: Job 1
`
	err := os.WriteFile(pipelineFile, []byte(pipelineContent), 0644)
	assert.NoError(t, err)

	exportPipelineGraphJob(pipelineFile, "", nil, "invalid", "-")

	assert.True(t, exitCalled)
	assert.Contains(t, buf.String(), "Unsupported graph format")
}

func TestExportPipelineGraphJob_WriteFileError(t *testing.T) {
	var buf bytes.Buffer
	stdout = &buf
	defer func() { stdout = os.Stdout }()

	exitCalled := false
	exitFunc = func(code int) {
		exitCalled = true
	}
	defer func() { exitFunc = os.Exit }()

	tmpDir := t.TempDir()
	pipelineFile := filepath.Join(tmpDir, "pipeline.yaml")
	pipelineContent := `
name: test-pipeline
jobs:
  job1:
    summary: Job 1
`
	err := os.WriteFile(pipelineFile, []byte(pipelineContent), 0644)
	assert.NoError(t, err)

	// provide a directory as output file to trigger error
	exportPipelineGraphJob(pipelineFile, "", nil, "mermaid", tmpDir)

	assert.True(t, exitCalled)
	assert.Contains(t, buf.String(), "Failed to write to file")
}
