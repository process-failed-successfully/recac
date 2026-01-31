package runner

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"recac/internal/db"
	"strings"
	"testing"
)

func TestSelectPrompt_DeterministicAssignment(t *testing.T) {
	// Setup workspace
	tmpDir, err := os.MkdirTemp("", "select_prompt_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create feature_list.json
	features := db.FeatureList{
		ProjectName: "Test Project",
		Features: []db.Feature{
			{
				ID:          "task-1",
				Description: "Task 1 Description [PRIMES]",
				Status:      "pending",
				Passes:      false,
				Dependencies: db.FeatureDependencies{
					ExclusiveWritePaths: []string{"file1"},
					ReadOnlyPaths:       []string{"file2"},
				},
			},
		},
	}
	data, _ := json.Marshal(features)
	if err := os.WriteFile(filepath.Join(tmpDir, "feature_list.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	// Create Session
	// We need a dummy agent
	mockAgent := agent.NewMockAgent()

	s := &Session{
		Workspace:        tmpDir,
		Agent:            mockAgent,
		Project:          "test-project",
		MaxIterations:    10,
		ManagerFrequency: 5,
		Iteration:        1,
		Logger:           slog.Default(),
		// No DB needed as it falls back to file
	}

	// Call SelectPrompt
	prompt, templateName, _, err := s.SelectPrompt()
	if err != nil {
		t.Fatalf("SelectPrompt failed: %v", err)
	}

	if templateName != "coding_agent" {
		t.Errorf("Expected coding_agent template, got %s", templateName)
	}

	// Check Prompt Content
	// We expect "Task 1 Description [PRIMES]"
	// The bug overwrites this with "Continue implementing..."

	if !strings.Contains(prompt, "Task 1 Description [PRIMES]") {
		t.Errorf("Prompt did not contain assigned task description. Prompt preview:\n%s", prompt[:min(len(prompt), 500)])
	}

	if strings.Contains(prompt, "Continue implementing pending features") {
		t.Errorf("Prompt contained generic fallback message, indicating vars were overwritten.")
	}
}
