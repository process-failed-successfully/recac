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

	// 5. Initialize DB Store (Required for signal setting)
	dbConfig := db.StoreConfig{
		Type:             "sqlite",
		ConnectionString: filepath.Join(tmpDir, "test.db"),
	}
	dbStore, err := db.NewStore(dbConfig)
	if err != nil {
		t.Fatalf("Failed to init DB: %v", err)
	}
	defer dbStore.Close()

	// 6. Initialize Session
	mockDocker := &MockDockerForExec{DB: dbStore}
	mockAgent := agent.NewMockAgent()
	s := &Session{
		Docker:           mockDocker,
		Agent:            mockAgent,
		Workspace:        tmpDir,
		FeatureContent:   features,
		ManagerFrequency: 5,
		Notifier:         notify.NewManager(func(string, ...interface{}) {}),
		Logger:           telemetry.NewLogger(true, "", false),
		DBStore:          dbStore,
		Project:          "test-project",
		AgentProvider:    "mock",
		AgentModel:       "mock-model",
	}

	// 7. Run Loop
	// We expect ErrNoOp if it continues after signaling COMPLETED because mock agent returns nothing.
	// However, if COMPLETED is signaled, it should transition to QA/Manager.
	// Since SkipQA/Mock, it might loop.
	// But crucially, it should NOT fail with "db store not initialized".
	err = s.RunLoop(context.Background())

	if err != nil && !errors.Is(err, ErrNoOp) && !errors.Is(err, ErrMaxIterations) {
		t.Errorf("RunLoop failed with unexpected error: %v", err)
	}

	// Verify COMPLETED signal was set
	val, err := dbStore.GetSignal("test-project", "COMPLETED")
	if err != nil || val != "true" {
		t.Errorf("Expected COMPLETED signal to be true, got %s (err: %v)", val, err)
	}
}
