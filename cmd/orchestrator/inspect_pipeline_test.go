package main

import (
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInspectPipelineJob(t *testing.T) {
	tests := []struct {
		name           string
		target         string
		vars           map[string]string
		expectedOutput []string
		expectedExit   int
		setupFile      func() string
	}{
		{
			name:   "Success with simple pipeline",
			target: "",
			vars:   map[string]string{"ENV_VAR": "my-value"},
			expectedOutput: []string{
				"Pipeline Inspection:",
				"Job ID:",
				"Summary:",
				"Build App",
				"Dependencies:",
				"setup",
				"Environment Vars:",
				"MY_VAR=my-value",
			},
			expectedExit: 0,
			setupFile: func() string {
				f, _ := os.CreateTemp("", "pipeline_*.yaml")
				f.Write([]byte(`
name: my-pipeline
jobs:
  setup:
    summary: Setup
    repo_url: https://github.com/org/repo
  build:
    summary: Build App
    depends_on: [setup]
    env_vars:
      MY_VAR: ${ENV_VAR}
`))
				f.Close()
				return f.Name()
			},
		},
		{
			name:   "Invalid file path",
			target: "",
			expectedOutput: []string{
				"Failed to read file nonexistent.yaml",
			},
			expectedExit: 1,
			setupFile: func() string {
				return "nonexistent.yaml"
			},
		},
		{
			name:   "Invalid YAML content",
			target: "",
			expectedOutput: []string{
				"Pipeline inspection failed:",
			},
			expectedExit: 1,
			setupFile: func() string {
				f, _ := os.CreateTemp("", "pipeline_*.yaml")
				f.Write([]byte(`
name: my-pipeline
jobs:
  - invalid_yaml_list_instead_of_map
`))
				f.Close()
				return f.Name()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var exitCode int
			oldExit := exitFunc
			exitFunc = func(code int) { exitCode = code }
			defer func() { exitFunc = oldExit }()

			filePath := tt.setupFile()
			if tt.name != "Invalid file path" {
				defer os.Remove(filePath)
			}

			oldStdout := stdout
			pr, pw, _ := os.Pipe()
			stdout = pw
			defer func() { stdout = oldStdout }()

			inspectPipelineJob(filePath, tt.target, tt.vars)

			pw.Close()
			outBytes, _ := io.ReadAll(pr)
			outStr := string(outBytes)

			for _, expected := range tt.expectedOutput {
				assert.Contains(t, outStr, expected)
			}
			assert.Equal(t, tt.expectedExit, exitCode)
		})
	}
}
