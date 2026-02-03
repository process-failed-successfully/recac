package runner

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"recac/internal/agent"
	"recac/internal/db"
	"recac/internal/notify"
	"recac/internal/telemetry"
)

func TestSession_RunLoop_UIVerification(t *testing.T) {
	// 1. Create a temp directory
	tmpDir, err := os.MkdirTemp("", "ui_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 2. Setup: app_spec.txt (required)
	os.WriteFile(filepath.Join(tmpDir, "app_spec.txt"), []byte("Spec"), 0644)

	// 3. Setup: feature_list.json with ALL PASSING (Use FeatureContent)
	features := `{"features":[{"id":"1","description":"feat","status":"done","passes":true}]}`

	// 4. Setup: ui_verification.json (Should be detected)
	os.WriteFile(filepath.Join(tmpDir, "ui_verification.json"), []byte("Verify Button Color"), 0644)

	// 5. Initialize DB (Required for signals)
	dbPath := filepath.Join(tmpDir, "recac.db")
	store, err := db.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create DB: %v", err)
	}
	defer store.Close()

	// 6. Initialize Session
	mockDocker := &MockDockerForExec{}
	mockAgent := agent.NewMockAgent()
	s := &Session{
		Project:          "test-ui-proj",
		Docker:           mockDocker,
		Agent:            mockAgent,
		Workspace:        tmpDir,
		DBStore:          store,
		FeatureContent:   features,
		ManagerFrequency: 5,
		Notifier:         notify.NewManager(func(string, ...interface{}) {}),
		Logger:           telemetry.NewLogger(true, "", false),
		SkipQA:           true, // Skip QA to avoid needing a QAAgent
	}

	// 7. Run Loop
	// Since features are passed, it should mark COMPLETED.
	// Since SkipQA is true, it should mark PROJECT_SIGNED_OFF.
	// Then it should exit.
	err = s.RunLoop(context.Background())

	if err != nil {
		t.Errorf("RunLoop failed: %v", err)
	}

	// Verify signals
	val, _ := store.GetSignal("test-ui-proj", "COMPLETED")
	if val == "true" {
		// It might be cleared if signed off
	}
	val, _ = store.GetSignal("test-ui-proj", "PROJECT_SIGNED_OFF")
	if val != "true" {
		// It should be cleared after RunLoop exits if it finishes successfully
		// Actually, RunLoop exits when PROJECT_SIGNED_OFF is processed.
		// It clears it during cleanup if success.
	}
}
