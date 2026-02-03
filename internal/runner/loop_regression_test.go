package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"recac/internal/db"
	"recac/internal/notify"
	"recac/internal/telemetry"
	"testing"
)

func TestSession_RunLoop_SuccessAtMaxIterations(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "app_spec.txt"), []byte("Spec"), 0644)
	dbPath := filepath.Join(tmpDir, ".recac.db")
	store, _ := db.NewSQLiteStore(dbPath)
	defer store.Close()

	mockDocker := &MockLoopDocker{}

	// Agent responses aren't crucial here as we are testing lifecycle flow via signals
	mockAgent := &MockLoopAgent{
		Responses: []string{"Working..."},
	}

	s := &Session{
		Workspace:        tmpDir,
		Docker:           mockDocker,
		Agent:            mockAgent,
		DBStore:          store,
		MaxIterations:    1, // Limit to 1 iteration
		Notifier:         notify.NewManager(func(string, ...interface{}) {}),
		Logger:           telemetry.NewLogger(true, "", false),
		Project:          "test-project",
		Iteration:        1, // Simulate we are already AT the limit (after 1st iteration)
	}

	// Pre-set PROJECT_SIGNED_OFF signal to simulate completion happening in the previous iteration
	if err := store.SetSignal("test-project", "PROJECT_SIGNED_OFF", "true"); err != nil {
		t.Fatalf("Failed to set signal: %v", err)
	}

	ctx := context.Background()
	err := s.RunLoop(ctx)

	// Current behavior: It should fail with ErrMaxIterations because it checks Iteration >= MaxIterations BEFORE checking signals.
	// Desired behavior: It should return nil (success) because signals are checked first.

	if err != nil {
		if errors.Is(err, ErrMaxIterations) {
			t.Log("Reproduced failure: RunLoop returned ErrMaxIterations despite PROJECT_SIGNED_OFF signal")
			t.Fail() // Fail the test to confirm reproduction (or pass if I invert logic? No, typically we write a failing test first)
		} else {
			t.Errorf("RunLoop failed with unexpected error: %v", err)
		}
	} else {
		t.Log("RunLoop succeeded (signal checked before max iterations)")
	}
}
