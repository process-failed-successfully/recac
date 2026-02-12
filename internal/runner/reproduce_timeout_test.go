package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"recac/internal/agent"
	"recac/internal/docker"
	"recac/internal/notify"
	"recac/internal/telemetry"
)

// MockDockerForReproduction mimics MockDockerForExec
type MockDockerForReproduction struct{}
func (m *MockDockerForReproduction) CheckDaemon(ctx context.Context) error { return nil }
func (m *MockDockerForReproduction) RunContainer(ctx context.Context, imageRef string, workspace string, extraBinds []string, env []string, user string) (string, error) { return "mock-id", nil }
func (m *MockDockerForReproduction) StopContainer(ctx context.Context, containerID string) error { return nil }
func (m *MockDockerForReproduction) Exec(ctx context.Context, containerID string, cmd []string) (string, error) { return "Success", nil }
func (m *MockDockerForReproduction) ExecAsUser(ctx context.Context, containerID string, user string, cmd []string) (string, error) { return "Success", nil }
func (m *MockDockerForReproduction) ImageExists(ctx context.Context, tag string) (bool, error) { return true, nil }
func (m *MockDockerForReproduction) ImageBuild(ctx context.Context, opts docker.ImageBuildOptions) (string, error) { return "mock-image-id", nil }
func (m *MockDockerForReproduction) PullImage(ctx context.Context, imageRef string) error { return nil }


func TestReproduceRunLoopTimeout(t *testing.T) {
	// 1. Create a temp directory
	tmpDir, err := os.MkdirTemp("", "ui_test_repro")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 2. Setup: app_spec.txt (required)
	os.WriteFile(filepath.Join(tmpDir, "app_spec.txt"), []byte("Spec"), 0644)

	// 3. Setup: feature_list.json with ALL PASSING
	features := `{"features":[{"id":"1","description":"feat","status":"done","passes":true}]}`

	// 4. Setup: ui_verification.json (Should be detected)
	os.WriteFile(filepath.Join(tmpDir, "ui_verification.json"), []byte("Verify Button Color"), 0644)

	// 5. Initialize Session
	mockDocker := &MockDockerForReproduction{}
	mockAgent := agent.NewMockAgent()
	s := &Session{
		Docker:           mockDocker,
		Agent:            mockAgent,
		Workspace:        tmpDir,
		FeatureContent:   features,
		ManagerFrequency: 5,
		Notifier:         notify.NewManager(func(string, ...interface{}) {}),
		Logger:           telemetry.NewLogger(true, "", false),
		MaxIterations:    20,
		// CRITICAL: Set AgentProvider to "mock" so RunLoop creates Mock agents for QA/Manager
		AgentProvider:    "mock",
		AgentModel:       "mock-model",
	}

	// 6. Run with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = s.RunLoop(ctx)

	// Analyze result
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Fatal("Test timed out! RunLoop is stuck in an infinite loop.")
	}

	// ErrNoOp is expected because the MockAgent eventually returns empty/generic responses after completion flow
	// Or nil if it exits cleanly.
	if err != nil && !errors.Is(err, ErrNoOp) && !errors.Is(err, ErrMaxIterations) {
		t.Errorf("RunLoop failed with unexpected error: %v", err)
	} else {
		t.Log("RunLoop completed successfully (or with expected exit).")
	}
}
