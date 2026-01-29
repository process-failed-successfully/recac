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
	// Create a temporary directory for test files
	tmpDir := t.TempDir()

	// Create a dummy feature_list.json
	featureListJSON := `{
		"project_name": "Test Project",
		"features": [
			{
				"id": "F-1",
				"category": "Core",
				"priority": "MVP",
				"description": "Base feature",
				"dependencies": {
					"depends_on_ids": []
				}
			},
			{
				"id": "F-2",
				"category": "UI",
				"priority": "POC",
				"description": "Dependent feature",
				"dependencies": {
					"depends_on_ids": ["F-1"]
				}
			}
		]
	}`

	featureListPath := filepath.Join(tmpDir, "feature_list.json")
	err := os.WriteFile(featureListPath, []byte(featureListJSON), 0644)
	require.NoError(t, err)

	// Execute the command
	cmd := &cobra.Command{}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err = runPlanVisualize(cmd, []string{featureListPath})
	require.NoError(t, err)

	output := buf.String()

	// Verify output contains Mermaid graph
	assert.Contains(t, output, "graph TD")
	// Check content (allowing for flexible HTML or simple quoting)
	assert.Contains(t, output, "F_1[\"<b>F-1</b><br/>Core<br/><i>Base feature</i>\"]:::mvp")
	assert.Contains(t, output, "F_2[\"<b>F-2</b><br/>UI<br/><i>Dependent feature</i>\"]:::poc")

	// Check dependency direction: F_1 is prerequisite for F_2
	assert.Contains(t, output, "F_1 --> F_2")

	// Check styles definitions
	assert.Contains(t, output, "classDef mvp")
	assert.Contains(t, output, "classDef poc")
}

func TestPlanVisualizeCmd_FileNotFound(t *testing.T) {
	cmd := &cobra.Command{}
	err := runPlanVisualize(cmd, []string{"nonexistent.json"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read feature list")
}

func TestPlanVisualizeCmd_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	featureListPath := filepath.Join(tmpDir, "invalid.json")
	err := os.WriteFile(featureListPath, []byte("invalid json"), 0644)
	require.NoError(t, err)

	cmd := &cobra.Command{}
	err = runPlanVisualize(cmd, []string{featureListPath})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse feature list")
}
