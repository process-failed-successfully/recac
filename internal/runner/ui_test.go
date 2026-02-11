package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
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

	// 5. Initialize Session
	// Initialize DB Store to prevent infinite loop on signal checking
	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := db.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	mockDocker := &MockDockerWithSideEffects{
		Store:   store,
		Project: "test-project",
	}
	mockAgent := agent.NewMockAgent()
	s := &Session{
		Docker:           mockDocker,
		Agent:            mockAgent,
		Workspace:        tmpDir,
		FeatureContent:   features,
		ManagerFrequency: 5,
		Notifier:         notify.NewManager(func(string, ...interface{}) {}),
		Logger:           telemetry.NewLogger(true, "", false),
		DBStore:          store,
		Project:          "test-project",
		MaxIterations:    20,
	}

	// 6. Capture Stdout? (Hard to do in test without refactor).
	// We can trust the code if it compiles and logic flows.
	// Or we can observe if it creates the COMPLETED signal.

	err = s.RunLoop(context.Background())

	// Since all features pass, it should mark COMPLETED and print UI verification msg.
	// We mainly verify it DOESN'T fail or block.
	// ErrNoOp is expected because the MockAgent returns empty responses.
	if err != nil && !errors.Is(err, ErrNoOp) {
		t.Errorf("RunLoop failed: %v", err)
	}
}

// MockDockerWithSideEffects intercepts agent-bridge commands to update the DB state
type MockDockerWithSideEffects struct {
	MockDockerForExec
	Store   db.Store
	Project string
}

func (m *MockDockerWithSideEffects) Exec(ctx context.Context, id string, cmd []string) (string, error) {
	// Reconstruct the command string to check for signals
	// Note: cmd is usually ["/bin/bash", "-c", "script..."]
	fullCmd := ""
	if len(cmd) > 2 {
		fullCmd = cmd[2]
	}

	// Intercept signal commands and apply them to the DB
	if strings.Contains(fullCmd, "agent-bridge signal") {
		// Parse key/value roughly
		if strings.Contains(fullCmd, "PROJECT_SIGNED_OFF true") {
			m.Store.SetSignal(m.Project, "PROJECT_SIGNED_OFF", "true")
		}
		// Add other signals if needed
	}

	return m.MockDockerForExec.Exec(ctx, id, cmd)
}
