package runner

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"recac/internal/agent"
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

	// 5. Setup: Pre-create PROJECT_SIGNED_OFF signal to ensure quick exit
	// This simulates that the Manager has already approved the project, avoiding the need for
	// complex mocking of QA/Manager agent flows and preventing infinite loops.
	os.WriteFile(filepath.Join(tmpDir, "PROJECT_SIGNED_OFF"), []byte("true"), 0644)

	// 6. Initialize Session
	mockDocker := &MockDockerForExec{}
	mockAgent := agent.NewMockAgent()
	s := &Session{
		Docker:           mockDocker,
		Agent:            mockAgent,
		Workspace:        tmpDir,
		FeatureContent:   features,
		ManagerFrequency: 5,
		MaxIterations:    10, // Safety limit
		Notifier:         notify.NewManager(func(string, ...interface{}) {}),
		Logger:           telemetry.NewLogger(true, "", false),
		AgentProvider:    "mock",
	}

	// 7. Run Loop
	err = s.RunLoop(context.Background())

	// It should exit successfully because PROJECT_SIGNED_OFF is present
	if err != nil {
		t.Errorf("RunLoop failed: %v", err)
	}
}
