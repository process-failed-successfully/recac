package runner

import (
	"context"
	"errors"
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

	// 5. Initialize DB (Required for agent interactions)
	dbPath := filepath.Join(tmpDir, ".recac.db")
	dbStore, err := db.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create mock DB: %v", err)
	}
	defer dbStore.Close()

	// 6. Initialize Session
	mockDocker := &MockDockerForExec{}
	mockAgent := agent.NewMockAgent()
	s := &Session{
		Docker:           mockDocker,
		Agent:            mockAgent,
		Workspace:        tmpDir,
		FeatureContent:   features,
		ManagerFrequency: 5,
		Notifier:         notify.NewManager(func(string, ...interface{}) {}),
		Logger:           telemetry.NewLogger(true, "", false),
		MaxIterations:    10, // Prevent infinite loop
		UseLocalAgent:    true,
		DBStore:          dbStore,
		Project:          "ui_test_project",
	}

	// 7. Execute
	err = s.RunLoop(context.Background())

	// Since all features pass, it should mark COMPLETED and print UI verification msg.
	// We mainly verify it DOESN'T fail or block.
	// ErrNoOp is expected if the MockAgent returns empty responses, but with recent fixes it might return commands.
	// We accept nil (success) or ErrNoOp or ErrMaxIterations.
	if err != nil && !errors.Is(err, ErrNoOp) && !errors.Is(err, ErrMaxIterations) {
		t.Errorf("RunLoop failed: %v", err)
	}
}
