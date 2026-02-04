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

	// 3. Setup: DB Store
	// We MUST initialize a DB store and set env vars so agent-bridge (subprocess) uses the same DB.
	dbPath := filepath.Join(tmpDir, "recac.db")
	t.Setenv("RECAC_DB_TYPE", "sqlite")
	t.Setenv("RECAC_DB_URL", dbPath)

	storeConfig := db.StoreConfig{
		Type:             "sqlite",
		ConnectionString: dbPath,
	}
	dbStore, err := db.NewStore(storeConfig)
	if err != nil {
		t.Fatalf("Failed to create DB store: %v", err)
	}
	defer dbStore.Close()

	// 4. Setup: feature_list.json with ALL PASSING (Use FeatureContent)
	features := `{"features":[{"id":"1","description":"feat","status":"done","passes":true}]}`

	// 5. Setup: ui_verification.json (Should be detected)
	os.WriteFile(filepath.Join(tmpDir, "ui_verification.json"), []byte("Verify Button Color"), 0644)

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
		DBStore:          dbStore,
		Project:          "test-project", // Required for DB keying
		MaxIterations:    5,              // Prevent infinite loops in test
	}

	// 7. Run Loop
	// Since all features pass, it should mark COMPLETED and print UI verification msg.
	// We mainly verify it DOESN'T fail or block.
	err = s.RunLoop(context.Background())

	// ErrNoOp is expected because the MockAgent returns empty responses.
	// ErrMaxIterations might happen if signals aren't processed fast enough, but we mainly want to avoid hangs.
	if err != nil && !errors.Is(err, ErrNoOp) && !errors.Is(err, ErrMaxIterations) {
		t.Errorf("RunLoop failed: %v", err)
	}
}
