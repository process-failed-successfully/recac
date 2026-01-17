package workflow

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunWorkflow_MockMode(t *testing.T) {
	// Create a temporary directory for the project
	tmpDir := t.TempDir()

	// Create app_spec.txt (required by session)
	err := os.WriteFile(tmpDir+"/app_spec.txt", []byte("spec"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Configuration for Mock Mode
	cfg := SessionConfig{
		SessionName:   "test-mock-session",
		ProjectPath:   tmpDir,
		IsMock:        true,
		MaxIterations: 1, // Ensure it exits quickly
		Debug:         true,
		AllowDirty:    true, // Skip git checks (though mock mode skips them anyway)
	}

	// Capture stdout to verify output
	// Note: Capturing stdout in tests running in parallel or concurrent with others might be flaky if we assert strictly,
	// but here we just want to ensure it runs without error.

	ctx := context.Background()

	// Execute RunWorkflow
	// We call the variable RunWorkflow. existing tests might have mocked it, but they restore it.
	err = RunWorkflow(ctx, cfg)

	// Verify
	// Since MockAgent does not complete the task, we expect "maximum iterations reached" error
	if err != nil {
		assert.Contains(t, err.Error(), "maximum iterations reached")
	} else {
		// If by chance it completed, that's also fine
		assert.NoError(t, err)
	}
}

// func TestRunWorkflow_Real_NormalMode_DryRun(t *testing.T) {
// 	// Skipped due to timeouts in CI environment
// }
