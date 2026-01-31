package runner

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"recac/internal/db"
	"strings"
	"testing"
)

func TestSelectPrompt_DeterministicAssignment(t *testing.T) {
	// Setup workspace
	tmpDir := t.TempDir()

	// Create feature_list.json
	features := db.FeatureList{
		ProjectName: "Test Project",
		Features: []db.Feature{
			{
				ID:          "feat-1",
				Description: "Implement login",
				Status:      "pending",
				Passes:      false,
				Dependencies: db.FeatureDependencies{
					ExclusiveWritePaths: []string{"auth.go"},
					ReadOnlyPaths:       []string{"main.go"},
				},
			},
		},
	}

	data, _ := json.Marshal(features)
	_ = os.WriteFile(filepath.Join(tmpDir, "feature_list.json"), data, 0644)

	// Mock Session
	s := &Session{
		Workspace:        tmpDir,
		Project:          "Test Project",
		AgentProvider:    "mock",
		Logger:           slog.Default(),
		ManagerFrequency: 5, // Prevent div by zero
		Iteration:        1,
		// We don't need DBStore for this test as loadFeatures falls back to file
	}

	// Call SelectPrompt
	prompt, _, _, err := s.SelectPrompt()
	if err != nil {
		t.Fatalf("SelectPrompt failed: %v", err)
	}

	// Verify Prompt Variables (Accounting for Markdown formatting)
	// Template: - **Feature ID**: {task_id}
	if !strings.Contains(prompt, "**Feature ID**: feat-1") {
		t.Errorf("Expected prompt to contain Feature ID 'feat-1', got:\n%s", prompt)
	}
	// Template: - **Description**: {task_description}
	if !strings.Contains(prompt, "**Description**: Implement login") {
		t.Errorf("Expected prompt to contain Description 'Implement login', got:\n%s", prompt)
	}

	// Check Fallback Prevention
	if strings.Contains(prompt, "Multiple/Not Assigned") {
		t.Errorf("Prompt contains fallback 'Multiple/Not Assigned' despite valid feature assignment")
	}
}
