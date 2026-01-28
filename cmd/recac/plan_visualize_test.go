package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlanVisualizeCmd(t *testing.T) {
	// Setup temporary workspace
	tmpDir, err := os.MkdirTemp("", "recac-plan-visualize-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	os.Chdir(tmpDir)

	// Create feature_list.json
	jsonContent := `{
		"project_name": "Test Project",
		"features": [
			{
				"id": "A",
				"description": "Feature A",
				"status": "done",
				"dependencies": { "depends_on_ids": [] }
			},
			{
				"id": "B",
				"description": "Feature B",
				"status": "pending",
				"dependencies": { "depends_on_ids": ["A"] }
			}
		]
	}`
	jsonPath := filepath.Join(tmpDir, "feature_list.json")
	err = os.WriteFile(jsonPath, []byte(jsonContent), 0644)
	require.NoError(t, err)

	// Run command
	cmd := &cobra.Command{
		RunE: RunVisualize,
	}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{jsonPath})

	err = cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "graph TD")
	assert.Contains(t, output, "A[\"Feature A\"]:::done")
	assert.Contains(t, output, "B[\"Feature B\"]:::pending")
	assert.Contains(t, output, "A --> B")
}

func TestPlanVisualizeCmd_MissingFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "recac-plan-visualize-test-missing")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	os.Chdir(tmpDir)

	cmd := &cobra.Command{
		RunE: RunVisualize,
	}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"non_existent.json"})

	err = cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read file")
}
