package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOptimizeCiCmd_NoArgsDefaultMissing(t *testing.T) {
	cmd := optimizeCiCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	err := runOptimizeCi(cmd, []string{})
	require.NoError(t, err)

	assert.Contains(t, buf.String(), "Default directory .github/workflows not found")
}

func TestOptimizeCiCmd_FileNotFound(t *testing.T) {
	cmd := optimizeCiCmd
	err := runOptimizeCi(cmd, []string{"/path/does/not/exist/workflows.yml"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to access")
}

func TestOptimizeCiCmd_ScanFiles(t *testing.T) {
	tmpDir := t.TempDir()

	workflowYAML := `
name: CI
on: [push]
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
`
	err := os.WriteFile(filepath.Join(tmpDir, "ci.yml"), []byte(workflowYAML), 0644)
	require.NoError(t, err)
    os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("hi"), 0644)

	cmd := optimizeCiCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	optCiJSON = true
	defer func() { optCiJSON = false }()

	err = runOptimizeCi(cmd, []string{tmpDir})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, `"file":`)
	assert.Contains(t, output, `missing_timeout`)
}

func TestOptimizeCiCmd_TextOutputAndIgnore(t *testing.T) {
	tmpDir := t.TempDir()

	workflowYAML := `
name: CI
on: [push]
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
`
	err := os.WriteFile(filepath.Join(tmpDir, "ci.yml"), []byte(workflowYAML), 0644)
	require.NoError(t, err)

	cmd := optimizeCiCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	optCiJSON = false
	optCiIgnore = "action_ref_tag" // Ignore the action pinning rule
	defer func() { optCiIgnore = "" }()

	err = runOptimizeCi(cmd, []string{tmpDir})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "missing_timeout")
	assert.NotContains(t, output, "action_ref_tag") // should be ignored
}

func TestOptimizeCiCmd_NoIssues(t *testing.T) {
	tmpDir := t.TempDir()

	workflowYAML := `
name: CI
on: [push]
permissions:
  contents: read
jobs:
  build:
    runs-on: ubuntu-latest
    timeout-minutes: 10
    steps:
      - uses: actions/checkout@a5ac7e51b41094c92402da3b24376905380afc29
`
	err := os.WriteFile(filepath.Join(tmpDir, "perfect.yml"), []byte(workflowYAML), 0644)
	require.NoError(t, err)

	cmd := optimizeCiCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	optCiJSON = false

	err = runOptimizeCi(cmd, []string{tmpDir})
	require.NoError(t, err)

	assert.Contains(t, buf.String(), "No issues found! CI configurations are optimized.")
}
