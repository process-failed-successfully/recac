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

	// Create a dummy feature_list.json
	featureListContent := `{
  "project_name": "Visualizer",
  "features": [
    {
      "id": "feat-1",
      "category": "core",
      "description": "Core Feature",
      "status": "done",
      "steps": [],
      "dependencies": {
        "depends_on_ids": []
      }
    },
    {
      "id": "feat-2",
      "category": "ui",
      "description": "UI Feature",
      "status": "pending",
      "steps": [],
      "dependencies": {
        "depends_on_ids": ["feat-1"]
      }
    }
  ]
}`
	jsonPath := filepath.Join(tmpDir, "feature_list.json")
	err = os.WriteFile(jsonPath, []byte(featureListContent), 0644)
	require.NoError(t, err)

	// Execute Plan Visualize Command using a fresh root to avoid routing issues
	cmd := NewPlanCmd()

	testRoot := &cobra.Command{Use: "test"}
	testRoot.AddCommand(cmd)
	testRoot.SetArgs([]string{"plan", "visualize", jsonPath})

	buf := new(bytes.Buffer)
	testRoot.SetOut(buf)
	testRoot.SetErr(buf)

	err = testRoot.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Assert Output
	assert.Contains(t, output, "graph TD")
	assert.Contains(t, output, "feat_1[\"Core Feature\"]:::done")
	assert.Contains(t, output, "feat_2[\"UI Feature\"]:::pending")
	assert.Contains(t, output, "feat_1 --> feat_2")
}

func TestPlanVisualizeCmd_Unicode(t *testing.T) {
	// Test truncation and unicode
	tmpDir, err := os.MkdirTemp("", "recac-vis-uni")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	os.Chdir(tmpDir)

	content := `{
  "project_name": "Uni",
  "features": [
    {
      "id": "f1",
      "status": "pending",
      "description": "こんにちは世界 This is a long description",
      "dependencies": {
		  "depends_on_ids": []
	  }
    }
  ]
}`
	err = os.WriteFile("list.json", []byte(content), 0644)
	require.NoError(t, err)

	cmd := NewPlanCmd()
	testRoot := &cobra.Command{Use: "test"}
	testRoot.AddCommand(cmd)
	testRoot.SetArgs([]string{"plan", "visualize", "list.json"})

	buf := new(bytes.Buffer)
	testRoot.SetOut(buf)
	testRoot.SetErr(buf)

	err = testRoot.Execute()
	require.NoError(t, err)

	// Check truncation
	// "こんにちは世界 This is a long description" is > 30 chars
	// It should be truncated and have "..."
	assert.Contains(t, buf.String(), "...")
}
