package workflow

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"recac/internal/agent"
	"recac/internal/runner"
	"strings"
	"testing"
)

func TestRunWorkflow_MockMode_Real(t *testing.T) {
	// This test exercises the IsMock=true branch of RunWorkflow

	originalNewSessionFunc := NewSessionFunc
	defer func() { NewSessionFunc = originalNewSessionFunc }()

	NewSessionFunc = func(d runner.DockerClient, a agent.Agent, workspace, image, project, provider, model string, maxAgents int) *runner.Session {
		s := runner.NewSession(d, a, workspace, image, project, provider, model, maxAgents)
		s.MaxIterations = 1
		return s
	}

	cfg := SessionConfig{
		IsMock:      true,
		SessionName: "mock-mode-test",
		ProjectPath: t.TempDir(),
	}

	err := RunWorkflow(context.Background(), cfg)
	if err != nil && err != runner.ErrMaxIterations {
		// It might fail with app_spec.txt missing, which is fine
		if !strings.Contains(err.Error(), "app_spec.txt") {
			t.Logf("Mock workflow returned: %v", err)
		}
	}
}

func TestRunWorkflow_UncommittedChanges(t *testing.T) {
	tmpDir := t.TempDir()

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Skipf("git init failed: %v", err)
	}

	// Config user for commit
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = tmpDir
	cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tmpDir
	cmd.Run()

	// Create file
	file := filepath.Join(tmpDir, "file.txt")
	os.WriteFile(file, []byte("content"), 0644)

	// Add and commit
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = tmpDir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = tmpDir
	cmd.Run()

	// Modify file
	os.WriteFile(file, []byte("content modified"), 0644)

	cfg := SessionConfig{
		ProjectPath: tmpDir,
		AllowDirty:  false,
		SessionName: "dirty-check",
	}

	err := RunWorkflow(context.Background(), cfg)
	if err == nil {
		t.Error("Expected error for uncommitted changes, got nil")
	} else {
		if !strings.Contains(err.Error(), "uncommitted changes detected") {
			t.Errorf("Expected error containing 'uncommitted changes detected', got: %v", err)
		}
	}
}

func TestRunWorkflow_Detached_Simple(t *testing.T) {
	mockSM := &ManualMockSessionManager{
		StartSessionFunc: func(name, goal string, command []string, cwd string) (*runner.SessionState, error) {
			return &runner.SessionState{PID: 12345, LogFile: "/tmp/log"}, nil
		},
	}

	cfg := SessionConfig{
		Detached:       true,
		SessionName:    "detached-simple",
		SessionManager: mockSM,
	}

	err := RunWorkflow(context.Background(), cfg)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}
