package main

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"recac/internal/db"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestImplementCmd(t *testing.T) {
	// Create temp dir
	tmpDir := t.TempDir()
	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	// Create a feature list
	features := db.FeatureList{
		ProjectName: "Test Project",
		Features: []db.Feature{
			{
				ID:          "FEAT-1",
				Description: "Implement login",
				Status:      "Pending",
				Steps:       []string{"Create handler", "Create template"},
			},
			{
				ID:          "FEAT-2",
				Description: "Implement logout",
				Status:      "Done", // Should be skipped
			},
		},
	}
	data, _ := json.Marshal(features)
	planFile := "feature_list.json"
	os.WriteFile(planFile, data, 0644)

	// Mock runWorkflowFunc
	originalRunWorkflowFunc := runWorkflowFunc
	defer func() { runWorkflowFunc = originalRunWorkflowFunc }()

	var capturedConfigs []SessionConfig
	runWorkflowFunc = func(ctx context.Context, cfg SessionConfig) error {
		capturedConfigs = append(capturedConfigs, cfg)
		return nil
	}

	// Create command
	cmd := &cobra.Command{Use: "test"}
	// Pass flags
	cmd.Flags().Bool("auto", true, "")
	cmd.Flags().String("from", "", "")
	cmd.Flags().Parse([]string{"--auto"})

	// Call runImplement
	err := runImplement(cmd, []string{planFile})
	assert.NoError(t, err)

	// Check if runWorkflow was called correctly
	assert.Len(t, capturedConfigs, 1, "Should have called workflow once for FEAT-1")
	assert.Equal(t, "impl-FEAT-1", capturedConfigs[0].SessionName)
	assert.Contains(t, capturedConfigs[0].Goal, "Implement Feature FEAT-1")

	// Check if file was updated
	content, _ := os.ReadFile(planFile)
	var updatedFeatures db.FeatureList
	json.Unmarshal(content, &updatedFeatures)

	assert.Equal(t, "Done", updatedFeatures.Features[0].Status)
	assert.True(t, updatedFeatures.Features[0].Passes)
	assert.Equal(t, "Done", updatedFeatures.Features[1].Status)
}

func TestImplementCmd_Resume(t *testing.T) {
	// Create temp dir
	tmpDir := t.TempDir()
	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	// Create a feature list
	features := db.FeatureList{
		ProjectName: "Test Project",
		Features: []db.Feature{
			{ID: "FEAT-1", Status: "Pending"},
			{ID: "FEAT-2", Status: "Pending"},
			{ID: "FEAT-3", Status: "Pending"},
		},
	}
	data, _ := json.Marshal(features)
	planFile := "feature_list.json"
	os.WriteFile(planFile, data, 0644)

	// Mock runWorkflowFunc
	originalRunWorkflowFunc := runWorkflowFunc
	defer func() { runWorkflowFunc = originalRunWorkflowFunc }()

	var capturedIDs []string
	runWorkflowFunc = func(ctx context.Context, cfg SessionConfig) error {
		// Extract ID from SessionName
		id := strings.TrimPrefix(cfg.SessionName, "impl-")
		capturedIDs = append(capturedIDs, id)
		return nil
	}

	// Create command
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().Bool("auto", true, "")
	cmd.Flags().String("from", "", "")
	cmd.Flags().Parse([]string{"--auto", "--from", "FEAT-2"})

	// Call runImplement
	err := runImplement(cmd, []string{planFile})
	assert.NoError(t, err)

	// Should execute FEAT-2 and FEAT-3, skip FEAT-1
	assert.Len(t, capturedIDs, 2)
	assert.Contains(t, capturedIDs, "FEAT-2")
	assert.Contains(t, capturedIDs, "FEAT-3")
	assert.NotContains(t, capturedIDs, "FEAT-1")
}
