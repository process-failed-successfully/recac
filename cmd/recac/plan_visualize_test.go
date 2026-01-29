package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"recac/internal/db"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlanVisualizeCmd(t *testing.T) {
	// 1. Setup Sample Data
	list := db.FeatureList{
		ProjectName: "Test Project",
		Features: []db.Feature{
			{
				ID:          "feat-1",
				Description: "Base feature",
				Status:      "completed",
			},
			{
				ID:          "feat-2",
				Description: "Dependent feature",
				Status:      "pending",
				Dependencies: db.FeatureDependencies{
					DependsOnIDs: []string{"feat-1"},
				},
			},
		},
	}

	data, err := json.Marshal(list)
	require.NoError(t, err)

	// 2. Write to temp file
	tmpDir, err := os.MkdirTemp("", "recac-plan-viz-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	inputFile := filepath.Join(tmpDir, "feature_list.json")
	err = os.WriteFile(inputFile, data, 0644)
	require.NoError(t, err)

	// 3. Test Standard Output
	t.Run("Output to Stdout", func(t *testing.T) {
		buf := new(bytes.Buffer)
		planVisualizeCmd.SetOut(buf)

		// Reset flags
		planVisualizeCmd.Flags().Set("output", "")

		// Call function directly to avoid cobra init baggage
		err := runPlanVisualize(planVisualizeCmd, []string{inputFile})
		require.NoError(t, err)

		out := buf.String()
		assert.Contains(t, out, "flowchart TD")
		assert.Contains(t, out, "subgraph Test_Project")
		assert.Contains(t, out, "feat_1[\"feat-1")
		assert.Contains(t, out, "feat_2[\"feat-2")
		assert.Contains(t, out, "feat_1 --> feat_2")
		// Check styles
		assert.Contains(t, out, "style feat_1 fill:#9f9") // completed = green
		assert.Contains(t, out, "style feat_2 fill:#eee") // pending = gray
	})

	// 4. Test File Output
	t.Run("Output to File", func(t *testing.T) {
		outputFile := filepath.Join(tmpDir, "graph.mermaid")

		buf := new(bytes.Buffer)
		planVisualizeCmd.SetOut(buf)
		planVisualizeCmd.Flags().Set("output", outputFile)

		err := runPlanVisualize(planVisualizeCmd, []string{inputFile})
		require.NoError(t, err)

		assert.Contains(t, buf.String(), "Graph saved to")

		// Verify file content
		content, err := os.ReadFile(outputFile)
		require.NoError(t, err)
		out := string(content)
		assert.Contains(t, out, "flowchart TD")
		assert.Contains(t, out, "feat_1 --> feat_2")
	})

	// 5. Test Missing File
	t.Run("Missing File", func(t *testing.T) {
		planVisualizeCmd.SetOut(new(bytes.Buffer)) // suppress output
		err := runPlanVisualize(planVisualizeCmd, []string{"non_existent.json"})
		assert.Error(t, err)
	})
}

func TestSanitizePlanID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"normal", "normal"},
		{"has space", "has_space"},
		{"dotted.id", "dotted_id"},
		{"mixed/sep", "mixed_sep"},
		{"(parens)", "_parens_"},
	}

	for _, tt := range tests {
		got := sanitizePlanID(tt.input)
		assert.Equal(t, tt.expected, got)
	}
}
