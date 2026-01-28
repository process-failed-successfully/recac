package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"recac/internal/db"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateMermaid(t *testing.T) {
	features := db.FeatureList{
		ProjectName: "TestProject",
		Features: []db.Feature{
			{
				ID:          "F1",
				Description: "Feature 1",
				Priority:    "High",
				Dependencies: db.FeatureDependencies{
					DependsOnIDs: []string{},
				},
			},
			{
				ID:          "F2",
				Description: "Feature 2",
				Priority:    "Low",
				Dependencies: db.FeatureDependencies{
					DependsOnIDs: []string{"F1"},
				},
			},
		},
	}

	mermaid := generateMermaidPlan(features)

	assert.Contains(t, mermaid, "graph TD")
	assert.Contains(t, mermaid, "title TestProject Plan")
	assert.Contains(t, mermaid, "F1[\"Feature 1<br/>(High)\"]")
	assert.Contains(t, mermaid, "F2[\"Feature 2<br/>(Low)\"]")
	assert.Contains(t, mermaid, "F1 --> F2")
	// Check styles
	assert.Contains(t, mermaid, "fill:#ffcccc") // High priority color (red-ish)
	assert.Contains(t, mermaid, "fill:#ccffcc") // Low priority color (green-ish)
}

func TestPlanVisualizeCmd(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "recac-plan-viz-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	os.Chdir(tmpDir)

	features := db.FeatureList{
		ProjectName: "TestProject",
		Features: []db.Feature{
			{
				ID:          "A",
				Description: "Feature A",
				Priority:    "Medium",
				Dependencies: db.FeatureDependencies{},
			},
		},
	}

	data, _ := json.Marshal(features)
	err = os.WriteFile("feature_list.json", data, 0644)
	require.NoError(t, err)

	// We need to execute the subcommand.
	// Since planCmd is global and modified by init(), we can use it, but it might have state from other tests.
	// Safer to construct a fresh command structure for testing or just call runPlanVisualize directly.

	// Testing the run function directly avoids Cobra global state issues
	cmd := &cobra.Command{}
	var outBuf strings.Builder
	cmd.SetOut(&outBuf)

	err = runPlanVisualize(cmd, []string{})
	require.NoError(t, err)

	output := outBuf.String()
	assert.Contains(t, output, "graph TD")
	assert.Contains(t, output, "A[\"Feature A<br/>(Medium)\"]")
}
