package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestComparePipelines_Identical(t *testing.T) {
	yamlContent := `
name: "TestPipeline"
jobs:
  job1:
    summary: "Job 1"
    repo_url: "https://example.com/repo1"
    agent_provider: "openai"
`
	tmpDir := t.TempDir()
	p1Path := filepath.Join(tmpDir, "p1.yaml")
	p2Path := filepath.Join(tmpDir, "p2.yaml")

	err := os.WriteFile(p1Path, []byte(yamlContent), 0644)
	assert.NoError(t, err)
	err = os.WriteFile(p2Path, []byte(yamlContent), 0644)
	assert.NoError(t, err)

	// Capture output
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	comparePipelines(p1Path + "," + p2Path)

	out := buf.String()
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, out, "Comparing Pipelines")
	assert.Contains(t, out, "Name")
	assert.Contains(t, out, "TestPipeline")
	assert.Contains(t, out, "identical")
	assert.NotContains(t, out, "present")
	assert.NotContains(t, out, "missing")
}

func TestComparePipelines_Modified(t *testing.T) {
	yamlContent1 := `
name: "TestPipeline"
jobs:
  job1:
    summary: "Job 1 Old"
    repo_url: "https://example.com/repo1"
    agent_provider: "openai"
    env_vars:
      VAR1: "val1"
  job2:
    summary: "Job 2"
`
	yamlContent2 := `
name: "TestPipeline"
jobs:
  job1:
    summary: "Job 1 New"
    repo_url: "https://example.com/repo2"
    agent_provider: "anthropic"
    env_vars:
      VAR1: "val2"
      VAR2: "new"
  job3:
    summary: "Job 3"
`

	tmpDir := t.TempDir()
	p1Path := filepath.Join(tmpDir, "p1.yaml")
	p2Path := filepath.Join(tmpDir, "p2.yaml")

	err := os.WriteFile(p1Path, []byte(yamlContent1), 0644)
	assert.NoError(t, err)
	err = os.WriteFile(p2Path, []byte(yamlContent2), 0644)
	assert.NoError(t, err)

	// Capture output
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	comparePipelines(p1Path + "," + p2Path)

	out := buf.String()
	assert.Equal(t, 0, exitCode)

	// Check job1 diffs
	assert.Contains(t, out, "Job 1 Old")
	assert.Contains(t, out, "Job 1 New")
	assert.Contains(t, out, "repo1")
	assert.Contains(t, out, "repo2")
	assert.Contains(t, out, "val1")
	assert.Contains(t, out, "val2")
	assert.Contains(t, out, "Env[VAR2]:")
	assert.Contains(t, out, "<missing>")

	// Check missing jobs
	assert.Contains(t, out, "Job: job2")
	assert.Contains(t, out, "Job: job3")
}

func TestComparePipelines_Error_InvalidInput(t *testing.T) {
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	comparePipelines("only_one_file.yaml")

	out := buf.String()
	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out, "Error: --compare-pipelines expects exactly two pipeline YAML files")
}

func TestComparePipelines_Error_FileNotFound(t *testing.T) {
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	comparePipelines("does_not_exist1.yaml,does_not_exist2.yaml")

	out := buf.String()
	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out, "Error parsing pipeline 1")
}
