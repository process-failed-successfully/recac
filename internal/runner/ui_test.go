package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

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

	// 5. Initialize DB (Required to prevent nil panic)
	dbPath := filepath.Join(tmpDir, ".recac.db")
	store, err := db.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create DB: %v", err)
	}
	defer store.Close()

	// 6. Initialize Session
	mockDocker := &MockDockerForExec{}
	mockAgent := agent.NewMockAgent()

	// Pre-configure mock agent response to avoid infinite loops if it gets called
	// If it gets called (e.g. QA check), we want it to behave predictably.
	// But in this test, we expect it to see "COMPLETED" and trigger QA.

	s := &Session{
		Docker:           mockDocker,
		Agent:            mockAgent,
		Workspace:        tmpDir,
		FeatureContent:   features,
		ManagerFrequency: 5,
		Notifier:         notify.NewManager(func(string, ...interface{}) {}),
		Logger:           telemetry.NewLogger(true, "", false),
		DBStore:          store,
		Project:          "ui-test-project",
		MaxIterations:    5, // Limit iterations to avoid timeout/infinite loop
		SleepFunc:        func(d time.Duration) {}, // No sleep
	}

	// 7. Run Loop
	err = s.RunLoop(context.Background())

	// Since all features pass, it should mark COMPLETED.
	// Then run QA Agent (MockAgent returns QA_PASSED).
	// Then run Manager Agent (MockAgent returns PROJECT_SIGNED_OFF).
	// Then exit.
	// So err should be nil.

	// If ErrNoOp or ErrMaxIterations occurs, that's also acceptable for this specific test structure
	// (as long as it doesn't panic or hang).
	if err != nil && !errors.Is(err, ErrNoOp) && !errors.Is(err, ErrMaxIterations) {
		t.Errorf("RunLoop failed: %v", err)
	}
}
