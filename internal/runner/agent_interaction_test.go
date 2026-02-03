package runner

import (
	"encoding/json"
	"path/filepath"
	"recac/internal/db"
	"recac/internal/telemetry"
	"strings"
	"testing"
)

func TestSelectPrompt_ConsumesUserHint(t *testing.T) {
	// Setup DB
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".recac.db")
	store, err := db.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Setup Session
	project := "test-project"
	s := &Session{
		Workspace:        tmpDir,
		Project:          project,
		DBStore:          store,
		Logger:           telemetry.NewLogger(true, "", false),
		ManagerFrequency: 10,
	}

	// Setup Feature List in DB (so it doesn't run Initializer)
	fl := db.FeatureList{
		ProjectName: project,
		Features: []db.Feature{
			{
				ID:          "feat-1",
				Description: "Original Description",
				Status:      "pending",
				Dependencies: db.FeatureDependencies{
					ExclusiveWritePaths: []string{"."},
					ReadOnlyPaths:       []string{},
				},
			},
		},
	}
	flBytes, _ := json.Marshal(fl)
	store.SaveFeatures(project, string(flBytes))

	// Test 1: No Hint
	s.SelectedTaskID = "feat-1" // Force selection
	prompt, _, _, err := s.SelectPrompt()
	if err != nil {
		t.Fatalf("SelectPrompt failed: %v", err)
	}
	if strings.Contains(prompt, "USER INTERVENTION") {
		t.Error("Prompt should not contain user intervention when no hint is set")
	}

	// Test 2: Set Hint
	hint := "Please check error handling."
	if err := store.SetSignal(project, "USER_HINT", hint); err != nil {
		t.Fatalf("Failed to set signal: %v", err)
	}

	// Verify hint is in prompt
	prompt, _, _, err = s.SelectPrompt()
	if err != nil {
		t.Fatalf("SelectPrompt failed: %v", err)
	}
	if !strings.Contains(prompt, "### USER INTERVENTION") {
		t.Error("Prompt should contain '### USER INTERVENTION'")
	}
	if !strings.Contains(prompt, hint) {
		t.Error("Prompt should contain the hint message")
	}

	// Test 3: Verify Hint is Consumed (deleted)
	val, err := store.GetSignal(project, "USER_HINT")
	if err != nil {
		t.Fatalf("GetSignal failed: %v", err)
	}
	if val != "" {
		t.Error("Signal should have been deleted after consumption")
	}

	// Test 4: Verify next prompt is clean
	prompt, _, _, err = s.SelectPrompt()
	if err != nil {
		t.Fatalf("SelectPrompt failed: %v", err)
	}
	if strings.Contains(prompt, "USER INTERVENTION") {
		t.Error("Next prompt should not contain user intervention")
	}
}
