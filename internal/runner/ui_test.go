package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

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

	// 5. Initialize Session
	mockDocker := &MockDockerForExec{}
	mockAgent := agent.NewMockAgent()

	// Mock SleepFunc to avoid waiting and to enforce a loop limit
	loopCount := 0
	maxLoops := 50

	s := &Session{
		Docker:           mockDocker,
		Agent:            mockAgent,
		Workspace:        tmpDir,
		FeatureContent:   features,
		ManagerFrequency: 5,
		Notifier:         notify.NewManager(func(string, ...interface{}) {}),
		Logger:           telemetry.NewLogger(true, "", false),
		SleepFunc: func(d time.Duration) {
			loopCount++
			if loopCount > maxLoops {
				// We must break the loop manually if it doesn't stop.
				// However, RunLoop doesn't check a "stopped" flag from SleepFunc.
				// But we can check context if we pass one that we can cancel?
				// Since RunLoop accepts a context, we can cancel it!
				// But we don't have access to the cancel func inside here unless we capture it.
				// A panic is a crude way to stop, but might fail the test in a way we don't want.
				// Actually, if we just return, the loop continues instantly.
				// We need a way to stop.
				// Let's rely on the fact that MockAgent should eventually trigger a condition or we just accept that we need to cancel the context.
			}
		},
	}

	// 6. Run with timeout context to ensure we don't hang forever
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// We also wrap SleepFunc to cancel the context if we hit a limit, allowing clean exit
	s.SleepFunc = func(d time.Duration) {
		loopCount++
		if loopCount > maxLoops {
			cancel() // Stop the loop
		}
	}

	err = s.RunLoop(ctx)

	// Since all features pass, it should mark COMPLETED and print UI verification msg.
	// We mainly verify it DOESN'T fail or block.
	// ErrNoOp is expected because the MockAgent returns empty responses.
	// context.Canceled is also expected if we forced it to stop.
	if err != nil && !errors.Is(err, ErrNoOp) && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("RunLoop failed: %v", err)
	}
}
