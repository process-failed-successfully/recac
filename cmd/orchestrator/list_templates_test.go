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
		name         string
		yamlContent  string
		vars         map[string]string
		expectOutput []string
		expectExit1  bool
	}{
		{
			name: "Valid Templates Full",
			yamlContent: `
name: Test Pipeline
templates:
  base-build:
    summary: Base Build
    description: Base Description
    task: make build
    extends: parent-tmpl
    stage: build
    repo_url: https://github.com/test/test
    run_condition: ALWAYS
    depends_on: [dep1]
    agent_provider: openrouter
    agent_model: openai/gpt-4o-mini
    tags: [build]
    priority: 10
    timeout: 10m
    delay: 5s
    concurrency_group: build-group
    cancel_in_progress: true
    max_retries: 3
    require_approval: true
    retry_delay: 10s
    retry_backoff_multiplier: 2.0
    env_vars:
      GOOS: linux
      GOARCH: amd64
    variables:
      VAR1: value1
    matrix:
      OS: [linux, windows]
jobs:
  job1:
    summary: App
`,
			vars:        nil,
			expectExit1: false,
			expectOutput: []string{
				"Template: base-build",
				"Summary:", "Base Build",
				"Description:", "Base Description",
				"Task:", "make build",
				"Extends:", "parent-tmpl",
				"Stage:", "build",
				"Repo URL:", "https://github.com/test/test",
				"Run Condition:", "ALWAYS",
				"Depends On:", "dep1",
				"Tags:", "build",
				"Priority:", "10",
				"Timeout:", "10m",
				"Delay:", "5s",
				"Concurrency Group:", "build-group",
				"Cancel In Progress:", "true",
				"Agent Provider:", "openrouter",
				"Agent Model:", "openai/gpt-4o-mini",
				"Max Retries:", "3",
				"Require Approval:", "true",
				"Retry Delay:", "10s",
				"Retry Backoff:", "2.00",
				"Env Vars:", "GOOS=linux", "GOARCH=amd64",
				"Variables:", "VAR1=value1",
				"Matrix:", "OS: [linux, windows]",
			},
		},
		{
			name: "No Templates Defined",
			yamlContent: `
name: Test Pipeline
jobs:
  job1:
    summary: App
`,
			vars:         nil,
			expectExit1:  false,
			expectOutput: []string{"No templates defined in pipeline"},
		},
		{
			name: "Invalid YAML",
			yamlContent: `
name: Invalid Yaml
templates:
  invalid: [
`,
			vars:         nil,
			expectExit1:  true,
			expectOutput: []string{"Failed to unmarshal pipeline YAML"},
		},
		{
			name:         "Missing File",
			yamlContent:  "", // We won't write this to file
			vars:         nil,
			expectExit1:  true,
			expectOutput: []string{"Failed to read file"},
		},
		{
			name: "Variables Substitution",
			yamlContent: `
name: Test Pipeline
templates:
  dynamic-tmpl:
    summary: ${TEMPLATE_SUMMARY}
    task: ${UNKNOWN_VAR}
jobs:
  job1:
    summary: App
`,
			vars:        map[string]string{"TEMPLATE_SUMMARY": "Dynamically Generated Summary"},
			expectExit1: false,
			expectOutput: []string{
				"Dynamically Generated Summary",
				"${UNKNOWN_VAR}", // Unsubstituted variable
			},
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
			for _, expected := range tt.expectOutput {
				assert.Contains(t, buf.String(), expected, "Output did not contain expected text")
			}
		})
	}
}
