package workflow

import (
	"context"
	"fmt"
	"os"
	"recac/internal/cmdutils"
	"recac/internal/git"
	"recac/internal/runner"
	"recac/internal/agent"
	"testing"
)

// ExtraMockSessionManager definition
type ExtraMockSessionManager struct {
	StartSessionFunc func(name, goal string, command []string, cwd string) (*runner.SessionState, error)
}

func (m *ExtraMockSessionManager) StartSession(name, goal string, command []string, cwd string) (*runner.SessionState, error) {
	if m.StartSessionFunc != nil {
		return m.StartSessionFunc(name, goal, command, cwd)
	}
	return &runner.SessionState{PID: 123, LogFile: "test.log"}, nil
}

func TestProcessDirectTask_Error(t *testing.T) {
	// Mock SetupWorkspace failure
	originalSetup := cmdutils.SetupWorkspace
	defer func() { cmdutils.SetupWorkspace = originalSetup }()
	cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
		return "", fmt.Errorf("setup failed")
	}

	cfg := SessionConfig{
		RepoURL: "http://repo",
		SessionName: "direct-error",
		IsMock: true,
	}

	err := ProcessDirectTask(context.Background(), cfg)
	if err == nil {
		t.Error("Expected error for SetupWorkspace failure")
	}
}

func TestRunWorkflow_Detached_AllFlags(t *testing.T) {
	sm := &ExtraMockSessionManager{
		StartSessionFunc: func(name, goal string, command []string, cwd string) (*runner.SessionState, error) {
			// Verify flags
			foundMock := false
			foundMaxIter := false
			foundManagerFreq := false
			foundTaskMaxIter := false
			foundAllowDirty := false

			for i, arg := range command {
				if arg == "--mock" { foundMock = true }
				if arg == "--max-iterations" && command[i+1] == "30" { foundMaxIter = true }
				if arg == "--manager-frequency" && command[i+1] == "10" { foundManagerFreq = true }
				if arg == "--task-max-iterations" && command[i+1] == "5" { foundTaskMaxIter = true }
				if arg == "--allow-dirty" { foundAllowDirty = true }
			}

			if !foundMock { t.Error("Missing --mock") }
			if !foundMaxIter { t.Error("Missing --max-iterations") }
			if !foundManagerFreq { t.Error("Missing --manager-frequency") }
			if !foundTaskMaxIter { t.Error("Missing --task-max-iterations") }
			if !foundAllowDirty { t.Error("Missing --allow-dirty") }

			return &runner.SessionState{PID: 12345, LogFile: "test.log"}, nil
		},
	}

	cfg := SessionConfig{
		Detached:          true,
		SessionName:       "detached-flags",
		SessionManager:    sm,
		IsMock:            true,
		MaxIterations:     30,
		ManagerFrequency:  10,
		TaskMaxIterations: 5,
		AllowDirty:        true,
		ProjectPath:       "/tmp/test",
	}

	err := RunWorkflow(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunWorkflow failed: %v", err)
	}
}

func TestRunWorkflow_Normal_WithEpic(t *testing.T) {
	// Mock NewSessionFunc
	originalNewSession := NewSessionFunc
	defer func() { NewSessionFunc = originalNewSession }()

	var session *runner.Session
	NewSessionFunc = func(d runner.DockerClient, a agent.Agent, workspace, image, project, provider, model string, maxAgents int) *runner.Session {
		s := runner.NewSession(d, a, workspace, image, project, provider, model, maxAgents)
		s.MaxIterations = 0 // Exit fast
		session = s
		return s
	}

	// Mock GetAgentClient
	originalGetAgent := cmdutils.GetAgentClient
	defer func() { cmdutils.GetAgentClient = originalGetAgent }()
	cmdutils.GetAgentClient = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return agent.NewMockAgent(), nil
	}

	cfg := SessionConfig{
		SessionName: "epic-test",
		JiraEpicKey: "EPIC-1",
		IsMock: false,
		AllowDirty: true,
		ProjectPath: t.TempDir(),
	}

	// Create app_spec.txt
	os.WriteFile(cfg.ProjectPath+"/app_spec.txt", []byte(""), 0644)

	RunWorkflow(context.Background(), cfg)

	if session != nil && session.BaseBranch != "agent-epic/EPIC-1" {
		t.Errorf("Expected BaseBranch agent-epic/EPIC-1, got %s", session.BaseBranch)
	}
}

func TestRunWorkflow_DefaultSessionName(t *testing.T) {
	// Mock dependencies to avoid side effects
	originalNewSession := NewSessionFunc
	defer func() { NewSessionFunc = originalNewSession }()

	NewSessionFunc = func(d runner.DockerClient, a agent.Agent, workspace, image, project, provider, model string, maxAgents int) *runner.Session {
		s := runner.NewSession(d, a, workspace, image, project, provider, model, maxAgents)
		s.MaxIterations = 0
		return s
	}

	// Mock GetAgentClient
	originalGetAgent := cmdutils.GetAgentClient
	defer func() { cmdutils.GetAgentClient = originalGetAgent }()
	cmdutils.GetAgentClient = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return agent.NewMockAgent(), nil
	}

	cfg := SessionConfig{
		// SessionName empty
		IsMock: false,
		AllowDirty: true,
		ProjectPath: t.TempDir(),
	}
	// Create spec
	os.WriteFile(cfg.ProjectPath+"/app_spec.txt", []byte(""), 0644)

	RunWorkflow(context.Background(), cfg)
}
