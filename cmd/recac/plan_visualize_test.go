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
	// Setup
	tmpDir, err := os.MkdirTemp("", "recac-plan-viz-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	jsonPath := filepath.Join(tmpDir, "feature_list.json")
	jsonContent := `{
      "project_name": "Test Project",
      "features": [
        {
          "id": "feat-1",
          "description": "Base feature",
          "status": "completed",
          "dependencies": { "depends_on_ids": [] }
        },
        {
          "id": "feat-2",
          "description": "Dependent feature",
          "status": "pending",
          "dependencies": { "depends_on_ids": ["feat-1"] }
        }
      ]
    }`
	err = os.WriteFile(jsonPath, []byte(jsonContent), 0644)
	require.NoError(t, err)

	// Execute
	// Since planVisualizeCmd is a global variable in main package, we can use it.
	// We reset stdout after test just in case, though usually SetOut is enough.

	buf := new(bytes.Buffer)
	planVisualizeCmd.SetOut(buf)

	err = runPlanVisualize(planVisualizeCmd, []string{jsonPath})
	require.NoError(t, err)

	output := buf.String()

	// Verify
	assert.Contains(t, output, "graph TD")
	assert.Contains(t, output, "feat_1") // sanitized ID
	assert.Contains(t, output, "feat_2")
	assert.Contains(t, output, "feat_1 --> feat_2")
	assert.Contains(t, output, "fill:#9f9") // Green for completed
}
