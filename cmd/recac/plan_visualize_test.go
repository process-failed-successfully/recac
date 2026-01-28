package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlanVisualizeCmd(t *testing.T) {
	// Setup temporary workspace
	tmpDir, err := os.MkdirTemp("", "recac-plan-viz-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	os.Chdir(tmpDir)

	// Create a dummy feature_list.json
	featureListJSON := `{
  "project_name": "Test Project",
  "features": [
    {
      "id": "feat-1",
      "description": "Feature 1",
      "status": "completed",
      "dependencies": {
        "depends_on_ids": []
      }
    },
    {
      "id": "feat-2",
      "description": "Feature 2",
      "status": "pending",
      "dependencies": {
        "depends_on_ids": ["feat-1"]
      }
    }
  ]
}`
	inputPath := filepath.Join(tmpDir, "feature_list.json")
	err = os.WriteFile(inputPath, []byte(featureListJSON), 0644)
	require.NoError(t, err)

	// Create command
	cmd := NewPlanVisualizeCmd()
	cmd.SetArgs([]string{inputPath})

	// Capture output
	var out bytes.Buffer
	cmd.SetOut(&out)

	// Run
	err = cmd.Execute()
	require.NoError(t, err)

	// Verify Output
	output := out.String()
	assert.Contains(t, output, "graph TD")
	assert.Contains(t, output, "feat_1")
	assert.Contains(t, output, "feat_2")
	// The sanitizer replaces - with _
	assert.Contains(t, output, "feat_1 --> feat_2")
	assert.Contains(t, output, "classDef done")
	assert.Contains(t, output, "classDef pending")
}

func TestPlanVisualizeCmd_Sanitization(t *testing.T) {
	// Setup temporary workspace
	tmpDir, err := os.MkdirTemp("", "recac-plan-viz-sanitize")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	inputPath := filepath.Join(tmpDir, "feature_list.json")

	// Feature IDs with special characters
	featureListJSON := `{
  "project_name": "Sanitize Project",
  "features": [
    {
      "id": "pkg/func",
      "description": "Complex ID",
      "status": "pending",
      "dependencies": {
        "depends_on_ids": []
      }
    }
  ]
}`
	err = os.WriteFile(inputPath, []byte(featureListJSON), 0644)
	require.NoError(t, err)

	cmd := NewPlanVisualizeCmd()
	cmd.SetArgs([]string{inputPath})

	var out bytes.Buffer
	cmd.SetOut(&out)

	err = cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	// pkg/func -> pkg_func
	assert.Contains(t, output, "pkg_func")
}
