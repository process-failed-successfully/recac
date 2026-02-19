package runner

import (
	"context"
	"os"
	"path/filepath"
	"recac/internal/db"
	"recac/internal/docker"
	"recac/internal/notify"
	"recac/internal/telemetry"
	"testing"
	"time"
)

// Minimal mocks for PlanOnly test

type MockPlanAgent struct {
	Response string
}

func (m *MockPlanAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, nil
}

func (m *MockPlanAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return m.Response, nil
}

type MockPlanDocker struct{}

func (m *MockPlanDocker) CheckDaemon(ctx context.Context) error { return nil }
func (m *MockPlanDocker) RunContainer(ctx context.Context, image, workspace string, binds, env, cmd []string, user string) (string, error) {
	return "mock-id", nil
}
func (m *MockPlanDocker) StopContainer(ctx context.Context, containerID string) error { return nil }
func (m *MockPlanDocker) Exec(ctx context.Context, containerID string, cmd []string) (string, error) {
	return "", nil
}
func (m *MockPlanDocker) ExecAsUser(ctx context.Context, containerID, user string, cmd []string) (string, error) {
	return "", nil
}
func (m *MockPlanDocker) PullImage(ctx context.Context, image string) error { return nil }
func (m *MockPlanDocker) ImageExists(ctx context.Context, image string) (bool, error) {
	return true, nil
}
func (m *MockPlanDocker) ImageBuild(ctx context.Context, options docker.ImageBuildOptions) (string, error) {
	return "mock-image", nil
}
func (m *MockPlanDocker) WaitContainer(ctx context.Context, containerID string) (int64, error) {
	return 0, nil
}

func TestSession_RunLoop_PlanOnly(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	specContent := "A simple todo app."
	specPath := filepath.Join(tmpDir, "app_spec.txt")
	os.WriteFile(specPath, []byte(specContent), 0644)

	dbPath := filepath.Join(tmpDir, ".recac.db")
	store, _ := db.NewSQLiteStore(dbPath)
	defer store.Close()

	mockDocker := &MockPlanDocker{}
	mockAgent := &MockPlanAgent{Response: "# Plan\n\n1. Do this.\n2. Do that."}

	s := &Session{
		Workspace:     tmpDir,
		SpecFile:      "app_spec.txt",
		Docker:        mockDocker,
		Agent:         mockAgent,
		DBStore:       store,
		Notifier:      notify.NewManager(func(string, ...interface{}) {}),
		Logger:        telemetry.NewLogger(true, "", false),
		PlanOnly:      true, // ENABLE PLAN ONLY
		SleepFunc:     func(d time.Duration) {},
	}

	ctx := context.Background()
	err := s.RunLoop(ctx)
	if err != nil {
		t.Errorf("RunLoop failed: %v", err)
	}

	// Verify PLAN.md exists
	planPath := filepath.Join(tmpDir, "PLAN.md")
	if _, err := os.Stat(planPath); os.IsNotExist(err) {
		t.Error("PLAN.md was not created")
	} else {
		content, _ := os.ReadFile(planPath)
		if string(content) != mockAgent.Response {
			t.Errorf("PLAN.md content mismatch. Got: %s, Want: %s", string(content), mockAgent.Response)
		}
	}
}
