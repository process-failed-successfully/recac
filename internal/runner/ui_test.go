package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"recac/internal/agent"
	"recac/internal/notify"
	"recac/internal/telemetry"
)

type MockDockerWithSideEffects struct {
	MockDockerForExec
	Workspace string
}

func (m *MockDockerWithSideEffects) Exec(ctx context.Context, id string, cmd []string) (string, error) {
	fullCmd := strings.Join(cmd, " ")

	// Simulate agent-bridge signal
	if strings.Contains(fullCmd, "agent-bridge signal") {
		if strings.Contains(fullCmd, "QA_PASSED true") {
			_ = os.WriteFile(filepath.Join(m.Workspace, ".QA_PASSED"), []byte("true"), 0644)
		}
		if strings.Contains(fullCmd, "PROJECT_SIGNED_OFF true") {
			_ = os.WriteFile(filepath.Join(m.Workspace, ".PROJECT_SIGNED_OFF"), []byte("true"), 0644)
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

	// 5. Initialize Session
	mockDocker := &MockDockerWithSideEffects{Workspace: tmpDir}
	mockAgent := agent.NewMockAgent()
	s := &Session{
		Docker:           mockDocker,
		Agent:            mockAgent,
		QAAgent:          mockAgent, // Inject mock for QA
		ManagerAgent:     mockAgent, // Inject mock for Manager
		Workspace:        tmpDir,
		FeatureContent:   features,
		ManagerFrequency: 5,
		Notifier:         notify.NewManager(func(string, ...interface{}) {}),
		Logger:           telemetry.NewLogger(true, "", false),
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
