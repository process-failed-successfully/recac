package tests

import (
	"os"
	"path/filepath"
	"recac/internal/db"
	"recac/internal/runner"
	"strings"
	"testing"
)

func TestSelectPrompt_InjectsHint(t *testing.T) {
	// 1. Setup
	tmpDir, err := os.MkdirTemp("", "recac-poke-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create app_spec.txt (required by SelectPrompt logic)
	if err := os.WriteFile(filepath.Join(tmpDir, "app_spec.txt"), []byte("Test Spec"), 0644); err != nil {
		t.Fatal(err)
	}

	// Initialize DB (Real Sqlite to avoid complex mocking)
	dbPath := filepath.Join(tmpDir, ".recac.db")
	store, err := db.NewStore(db.StoreConfig{Type: "sqlite", ConnectionString: dbPath})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	project := "test-project"

	// 2. Create Session
	session := runner.NewSessionWithConfig(tmpDir, project, "mock", "mock", store)
	session.ManagerFrequency = 100 // Avoid manager review triggering
	session.Iteration = 1          // Start at iteration 1

	// 3. Set Signal
	hint := "Do not forget to handle edge cases!"
	if err := store.SetSignal(project, "USER_HINT", hint); err != nil {
		t.Fatal(err)
	}

	// 4. Call SelectPrompt
	// We need to make sure SelectPrompt goes to CodingAgent path.
	// It checks for Initializer first.
	// If features exist, it skips Initializer.
	// So let's create a feature list.
	featuresJSON := `{"project_name": "test-project", "features": [{"id": "1", "description": "Test Feature", "status": "pending", "dependencies": {"depends_on_ids": [], "exclusive_write_paths": [], "read_only_paths": []}}]}`
	if err := os.WriteFile(filepath.Join(tmpDir, "feature_list.json"), []byte(featuresJSON), 0644); err != nil {
		t.Fatal(err)
	}

	// Create logger to prevent panic if logger is used
	// session.Logger should be initialized by NewSessionWithConfig but let's be safe
	// NewSessionWithConfig initializes logger.

	prompt, _, _, err := session.SelectPrompt()
	if err != nil {
		t.Fatalf("SelectPrompt failed: %v", err)
	}

	// 5. Verify Prompt
	if !strings.Contains(prompt, "### USER INTERVENTION") {
		t.Errorf("Prompt should contain user intervention header. Got:\n%s", prompt)
	}
	if !strings.Contains(prompt, hint) {
		t.Errorf("Prompt should contain hint: %s", hint)
	}

	// 6. Verify Signal Consumed
	val, err := store.GetSignal(project, "USER_HINT")
	if err == nil && val != "" {
		t.Errorf("Signal should have been deleted, but got: %s", val)
	}
}
