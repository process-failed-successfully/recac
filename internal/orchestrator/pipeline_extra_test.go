package orchestrator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveAndMergeIncludes(t *testing.T) {
	// Setup a temporary directory for sandbox
	sandboxDir := t.TempDir()

	// Helper function to create files in the sandbox
	createFile := func(name string, content string) string {
		path := filepath.Join(sandboxDir, name)
		err := os.MkdirAll(filepath.Dir(path), 0755)
		require.NoError(t, err)
		err = os.WriteFile(path, []byte(content), 0644)
		require.NoError(t, err)
		return path
	}

	validIncludeFile := createFile("valid_include.yaml", `
jobs:
  included-job:
    summary: "Included Job"
templates:
  included-template:
    summary: "Included Template"
variables:
  INCLUDED_VAR: "included_value"
secrets:
  - INCLUDED_SECRET
stages:
  - included_stage
`)

	recursiveIncludeFile := createFile("recursive_include.yaml", `
include:
  - child_include.yaml
jobs:
  recursive-job:
    summary: "Recursive Job"
`)

	createFile("child_include.yaml", `
jobs:
  child-job:
    summary: "Child Job"
`)

	invalidYamlFile := createFile("invalid_yaml.yaml", `
jobs:
  job1:
    - this
    - is: invalid
`)

	// LFI tests
	outOfBoundsFile := filepath.Join(t.TempDir(), "out_of_bounds.yaml")
	os.WriteFile(outOfBoundsFile, []byte("jobs:\n  oob-job:\n    summary: OOB"), 0644)

	lfiIncludeFile := createFile("lfi_include.yaml", `
include:
  - ../../../../out_of_bounds.yaml
`)

	varsIncludeFile := createFile("vars_include.yaml", `
variables:
  VARS_JOB: "Job ${VAR_TEST}"
jobs:
  var-job:
    summary: "${VARS_JOB}"
`)

	missingVarsIncludeFile := createFile("missing_vars_include.yaml", `
variables:
  VARS_JOB: "Job ${VAR_MISSING}"
jobs:
  var-job:
    summary: "${VAR_MISSING}"
`)

	tests := []struct {
		name          string
		mainPipeline  Pipeline
		includeFile   string
		vars          map[string]string
		expectError   bool
		errorContains string
		verify        func(t *testing.T, p *Pipeline)
	}{
		{
			name:        "Missing include file",
			includeFile: filepath.Join(sandboxDir, "missing.yaml"),
			expectError: true,
		},
		{
			name:          "Invalid YAML file",
			includeFile:   invalidYamlFile,
			expectError:   true,
			errorContains: "yaml: unmarshal errors",
		},
		{
			name:        "Valid include merging",
			includeFile: validIncludeFile,
			mainPipeline: Pipeline{
				Jobs: map[string]PipelineJob{
					"main-job": {Summary: "Main Job"},
				},
				Templates: map[string]PipelineJob{
					"main-template": {Summary: "Main Template"},
				},
				Variables: map[string]string{
					"MAIN_VAR": "main_value",
				},
				Secrets: []string{"MAIN_SECRET", "INCLUDED_SECRET"},
				Stages:  []string{"main_stage"},
			},
			expectError: false,
			verify: func(t *testing.T, p *Pipeline) {
				assert.Len(t, p.Jobs, 2)
				assert.Contains(t, p.Jobs, "main-job")
				assert.Contains(t, p.Jobs, "included-job")

				assert.Len(t, p.Templates, 2)
				assert.Contains(t, p.Templates, "main-template")
				assert.Contains(t, p.Templates, "included-template")

				assert.Len(t, p.Variables, 2)
				assert.Equal(t, "main_value", p.Variables["MAIN_VAR"])
				assert.Equal(t, "included_value", p.Variables["INCLUDED_VAR"])

				assert.Len(t, p.Secrets, 2)
				assert.Contains(t, p.Secrets, "MAIN_SECRET")
				assert.Contains(t, p.Secrets, "INCLUDED_SECRET") // Check deduplication

				assert.Len(t, p.Stages, 2)
				assert.Equal(t, "included_stage", p.Stages[0]) // Prepended
				assert.Equal(t, "main_stage", p.Stages[1])
			},
		},
		{
			name:        "Recursive includes",
			includeFile: recursiveIncludeFile,
			mainPipeline: Pipeline{},
			expectError: false,
			verify: func(t *testing.T, p *Pipeline) {
				assert.Len(t, p.Jobs, 2)
				assert.Contains(t, p.Jobs, "recursive-job")
				assert.Contains(t, p.Jobs, "child-job")
			},
		},
		{
			name:          "LFI check",
			includeFile:   lfiIncludeFile,
			mainPipeline: Pipeline{},
			expectError:   true,
			errorContains: "invalid recursive include path",
		},
		{
			name:        "Variable expansion",
			includeFile: varsIncludeFile,
			mainPipeline: Pipeline{},
			vars: map[string]string{
				"VAR_TEST": "Expanded",
			},
			expectError: false,
			verify: func(t *testing.T, p *Pipeline) {
				assert.Equal(t, "Job Expanded", p.Variables["VARS_JOB"])
				assert.Equal(t, "${VARS_JOB}", p.Jobs["var-job"].Summary)
			},
		},
		{
			name:        "Missing variable preserved",
			includeFile: missingVarsIncludeFile,
			mainPipeline: Pipeline{},
			vars: map[string]string{
				"OTHER_VAR": "Val",
			},
			expectError: false,
			verify: func(t *testing.T, p *Pipeline) {
				assert.Equal(t, "Job ${VAR_MISSING}", p.Variables["VARS_JOB"])
				assert.Equal(t, "${VAR_MISSING}", p.Jobs["var-job"].Summary)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := tt.mainPipeline
			err := resolveAndMergeIncludes(&p, tt.includeFile, sandboxDir, tt.vars)
			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
				if tt.verify != nil {
					tt.verify(t, &p)
				}
			}
		})
	}
}
