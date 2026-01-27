package workflow

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"recac/internal/agent"
	"recac/internal/cmdutils"
	"recac/internal/docker"
	"recac/internal/runner"
	"recac/internal/telemetry"
)

func TestRunWorkflow_Detached_Success(t *testing.T) {
	// Setup
	expectedSession := &runner.SessionState{PID: 1234, LogFile: "test.log"}
	called := false
	mockSM := &ManualMockSessionManager{
		StartSessionFunc: func(name, goal string, command []string, cwd string) (*runner.SessionState, error) {
			called = true
			if name != "test-session" {
				t.Errorf("Expected session name 'test-session', got '%s'", name)
			}
			return expectedSession, nil
		},
	}

	cfg := SessionConfig{
		Detached:       true,
		SessionName:    "test-session",
		Goal:           "test-goal",
		SessionManager: mockSM,
		ProjectPath:    "/tmp/test",
	}

	// Execute
	err := RunWorkflow(context.Background(), cfg)

	// Verify
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if !called {
		t.Error("Expected StartSession to be called")
	}
}

func TestRunWorkflow_Detached_Options(t *testing.T) {
	// Setup
	expectedSession := &runner.SessionState{PID: 1234, LogFile: "test.log"}
	mockSM := &ManualMockSessionManager{
		StartSessionFunc: func(name, goal string, command []string, cwd string) (*runner.SessionState, error) {
			// Verify flags
			args := strings.Join(command, " ")
			if !strings.Contains(args, "--mock") {
				t.Error("Expected --mock flag")
			}
			if !strings.Contains(args, "--allow-dirty") {
				t.Error("Expected --allow-dirty flag")
			}
			if !strings.Contains(args, "--max-iterations 100") {
				t.Error("Expected --max-iterations 100")
			}
			if !strings.Contains(args, "--manager-frequency 10") {
				t.Error("Expected --manager-frequency 10")
			}
			if !strings.Contains(args, "--task-max-iterations 50") {
				t.Error("Expected --task-max-iterations 50")
			}
			if !strings.Contains(args, "--path /tmp/test") {
				t.Error("Expected --path /tmp/test")
			}
			return expectedSession, nil
		},
	}

	cfg := SessionConfig{
		Detached:          true,
		SessionName:       "test-session",
		Goal:              "test-goal",
		SessionManager:    mockSM,
		ProjectPath:       "/tmp/test",
		IsMock:            true,
		AllowDirty:        true,
		MaxIterations:     100,
		ManagerFrequency:  10,
		TaskMaxIterations: 50,
	}

	// Execute
	err := RunWorkflow(context.Background(), cfg)

	// Verify
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestRunWorkflow_Detached_Fail(t *testing.T) {
	// Setup
	mockSM := &ManualMockSessionManager{
		StartSessionFunc: func(name, goal string, command []string, cwd string) (*runner.SessionState, error) {
			return nil, errors.New("start failed")
		},
	}

	cfg := SessionConfig{
		Detached:       true,
		SessionName:    "test-session",
		Goal:           "test-goal",
		SessionManager: mockSM,
	}

	// Execute
	err := RunWorkflow(context.Background(), cfg)

	// Verify
	if err == nil {
		t.Error("Expected error, got nil")
	} else if !strings.Contains(err.Error(), "start failed") {
		t.Errorf("Expected error containing 'start failed', got '%v'", err)
	}
}


func TestRunWorkflow_NormalMode_Mocked(t *testing.T) {
	// Mock NewSessionFunc
	originalNewSession := NewSessionFunc
	defer func() { NewSessionFunc = originalNewSession }()

	// Mock cmdutils.GetAgentClient
	originalGetAgentClient := cmdutils.GetAgentClient
	defer func() { cmdutils.GetAgentClient = originalGetAgentClient }()

	mockDocker, _ := docker.NewMockClient()
	mockAgent := agent.NewMockAgent()
	mockAgent.SetResponse("exit")

	cmdutils.GetAgentClient = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	NewSessionFunc = func(d runner.DockerClient, a agent.Agent, workspace, image, project, provider, model string, maxAgents int) *runner.Session {
		s := runner.NewSession(mockDocker, mockAgent, workspace, image, project, provider, model, maxAgents)
		s.MaxIterations = 1
		s.Logger = telemetry.NewLogger(true, "", false)

		// To avoid "CRITICAL ERROR: app_spec.txt not found"
		os.WriteFile(filepath.Join(workspace, "app_spec.txt"), []byte("spec"), 0644)

		return s
	}

	tmpDir := t.TempDir()

	// Create git repo to pass pre-flight check
	os.MkdirAll(filepath.Join(tmpDir, ".git"), 0755)

	cfg := SessionConfig{
		IsMock:        false,
		SessionName:   "normal-run",
		ProjectPath:   tmpDir,
		Provider:      "mock",
		MaxIterations: 1,
		AllowDirty:    true, // Skip git checks
	}

	err := RunWorkflow(context.Background(), cfg)

	if err != runner.ErrMaxIterations {
		t.Errorf("Expected ErrMaxIterations, got %v", err)
	}
}
