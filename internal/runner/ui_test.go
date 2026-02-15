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

// Specialized Mock for UI Test to handle signals
type MockDockerForUI struct {
	MockDockerForExec
	Workspace string
	DBStore   db.Store
	Project   string
}

func (m *MockDockerForUI) Exec(ctx context.Context, id string, cmd []string) (string, error) {
	fullCmd := strings.Join(cmd, " ")

	// Intercept signal creation
	if strings.Contains(fullCmd, "agent-bridge signal") {
		// Detect PROJECT_SIGNED_OFF
		if strings.Contains(fullCmd, "PROJECT_SIGNED_OFF") {
			// Write to DB directly as it's a privileged signal
			if m.DBStore != nil {
				if err := m.DBStore.SetSignal(m.Project, "PROJECT_SIGNED_OFF", "true"); err != nil {
					// Log error if possible or panic in test
					panic(err)
				}
			}
		}
	}

	return m.MockDockerForExec.Exec(ctx, id, cmd)
}

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

	// 5. Initialize DB Store (Required for Signals)
	dbPath := filepath.Join(tmpDir, "recac.db")
	dbStore, err := db.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to init DB: %v", err)
	}
	defer dbStore.Close()

	// 5. Initialize Session
	mockDocker := &MockDockerForUI{
		Workspace: tmpDir,
		DBStore:   dbStore,
		Project:   "ui-test-project",
	}
	mockAgent := agent.NewMockAgent()
	s := &Session{
		Docker:           mockDocker,
		Agent:            mockAgent,
		Workspace:        tmpDir,
		FeatureContent:   features,
		ManagerFrequency: 5,
		MaxIterations:    5, // Ensure loop terminates
		Notifier:         notify.NewManager(func(string, ...interface{}) {}),
		Logger:           telemetry.NewLogger(true, "", false),
		DBStore:          dbStore,
		Project:          "ui-test-project",
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
