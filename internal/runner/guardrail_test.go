package runner

import (
	"context"
	"os"
	"recac/internal/notify"
	"recac/internal/telemetry"
	"strings"
	"testing"
)

func TestSession_RunLoop_MissingSpec(t *testing.T) {
	// 1. Create a temp directory (empty workspace)
	tmpDir, err := os.MkdirTemp("", "guardrail_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 2. Initialize Session
	mockDocker := &MockDockerForExec{} // Re-using the mock from agent_exec_test.go
	s := &Session{
		Docker:    mockDocker,
		Workspace: tmpDir,
		Notifier:  notify.NewManager(func(string, ...interface{}) {}),
		Logger:    telemetry.NewLogger(true, "", false),
	}

	// 3. Run Loop
	err = s.RunLoop(context.Background())

	// 4. Assert Failure
	if err == nil {
		t.Error("Expected RunLoop to fail due to missing app_spec.txt, but it succeeded")
	} else {
		if !strings.Contains(err.Error(), "app_spec.txt not found") {
			t.Errorf("Expected error about app_spec.txt, got: %v", err)
		}
	}
}
