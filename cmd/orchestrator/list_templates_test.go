package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListTemplatesJob(t *testing.T) {
	// Table-driven tests
	tests := []struct {
		name          string
		yamlContent   string
		vars          map[string]string
		expectOutput  string
		expectExit1   bool
	}{
		{
			name: "Valid Templates",
			yamlContent: `
name: Test Pipeline
templates:
  base-build:
    summary: Base Build
    task: make build
    agent_provider: openrouter
    agent_model: openai/gpt-4o-mini
    tags: [build]
    env_vars:
      GOOS: linux
      GOARCH: amd64
    max_retries: 3
    timeout: 10m
  base-test:
    summary: Base Test
    extends: base-build
    task: make test
    tags: [test]
jobs:
  job1:
    summary: App
`,
			vars: nil,
			expectExit1: false,
			expectOutput: "Template: base-build",
		},
		{
			name: "No Templates Defined",
			yamlContent: `
name: Test Pipeline
jobs:
  job1:
    summary: App
`,
			vars: nil,
			expectExit1: false,
			expectOutput: "No templates defined in pipeline",
		},
		{
			name: "Invalid YAML",
			yamlContent: `
name: Invalid Yaml
templates:
  invalid: [
`,
			vars: nil,
			expectExit1: true,
			expectOutput: "Failed to unmarshal pipeline YAML",
		},
		{
			name: "Missing File",
			yamlContent: "", // We won't write this to file
			vars: nil,
			expectExit1: true,
			expectOutput: "Failed to read file",
		},
		{
			name: "Variables Substitution",
			yamlContent: `
name: Test Pipeline
templates:
  dynamic-tmpl:
    summary: ${TEMPLATE_SUMMARY}
jobs:
  job1:
    summary: App
`,
			vars: map[string]string{"TEMPLATE_SUMMARY": "Dynamically Generated Summary"},
			expectExit1: false,
			expectOutput: "Dynamically Generated Summary",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock stdout and exitFunc
			var buf bytes.Buffer
			oldStdout := stdout
			stdout = &buf
			defer func() { stdout = oldStdout }()

			exitCalled := false
			oldExitFunc := exitFunc
			exitFunc = func(code int) {
				exitCalled = true
			}
			defer func() { exitFunc = oldExitFunc }()

			// Create temp file if content is provided
			filePath := "non-existent-file.yaml"
			if tt.name != "Missing File" {
				tmpDir := t.TempDir()
				filePath = filepath.Join(tmpDir, "pipeline.yaml")
				err := os.WriteFile(filePath, []byte(tt.yamlContent), 0644)
				require.NoError(t, err)
			}

			// Run the function
			listTemplatesJob(filePath, tt.vars)

			// Assertions
			assert.Equal(t, tt.expectExit1, exitCalled, "exitFunc behavior did not match expectations")
			assert.Contains(t, buf.String(), tt.expectOutput, "Output did not contain expected text")
		})
	}
}
