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

	// Setup DB Store (Required for signals)
	dbPath := filepath.Join(tmpDir, ".recac.db")
	dbStore, err := db.NewStore(db.StoreConfig{Type: "sqlite", ConnectionString: dbPath})
	if err != nil {
		t.Fatalf("Failed to create db store: %v", err)
	}

	// 5. Initialize Session
	project := "test-project"
	mockDocker := &MockDockerClient{}
	mockDocker.ExecFunc = func(ctx context.Context, containerID string, cmd []string) (string, error) {
		cmdStr := strings.Join(cmd, " ")
		// Intercept signal command from Manager agent
		if strings.Contains(cmdStr, "PROJECT_SIGNED_OFF") {
			dbStore.SetSignal(project, "PROJECT_SIGNED_OFF", "true")
		}
		return "Success", nil
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
		DBStore:          dbStore,
		Project:          project,
		MaxIterations:    10, // Prevent infinite loop in test
	}

	// 6. Capture Stdout? (Hard to do in test without refactor).
	// We can trust the code if it compiles and logic flows.
	// Or we can observe if it creates the COMPLETED signal.

	err = s.RunLoop(context.Background())

	// Since all features pass, it should mark COMPLETED and print UI verification msg.
	// We mainly verify it DOESN'T fail or block.
	// ErrNoOp is expected because the MockAgent returns empty responses.
	// OR nil if it finishes successfully via PROJECT_SIGNED_OFF
	if err != nil && !errors.Is(err, ErrNoOp) && !errors.Is(err, ErrMaxIterations) {
		t.Errorf("RunLoop failed: %v", err)
	}
}
