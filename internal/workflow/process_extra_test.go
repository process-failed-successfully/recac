package workflow

import (
	"context"
	"errors"
	"recac/internal/agent"
	"recac/internal/cmdutils"
	"recac/internal/git"
	"recac/internal/runner"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProcessDirectTask_SetupFailure(t *testing.T) {
	origSetupWorkspace := cmdutils.SetupWorkspace
	defer func() { cmdutils.SetupWorkspace = origSetupWorkspace }()

	cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
		return "", errors.New("setup failed")
	}

	cfg := SessionConfig{
		SessionName: "test-session",
		RepoURL:     "https://github.com/test/repo",
	}

	err := ProcessDirectTask(context.Background(), cfg)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "setup failed")
}

func TestProcessDirectTask_RunFailure(t *testing.T) {
	origSetupWorkspace := cmdutils.SetupWorkspace
	origRunWorkflow := RunWorkflow
	defer func() {
		cmdutils.SetupWorkspace = origSetupWorkspace
		RunWorkflow = origRunWorkflow
	}()

	cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
		return repoURL, nil
	}

	RunWorkflow = func(ctx context.Context, cfg SessionConfig) error {
		return errors.New("run failed")
	}

	cfg := SessionConfig{
		SessionName: "test-session",
		RepoURL:     "https://github.com/test/repo",
	}

	err := ProcessDirectTask(context.Background(), cfg)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "run failed")
}

func TestRunWorkflow_Normal_AgentInitFailure(t *testing.T) {
	// Mock cmdutils.GetAgentClient to fail
	origGetAgentClient := cmdutils.GetAgentClient
	defer func() { cmdutils.GetAgentClient = origGetAgentClient }()

	cmdutils.GetAgentClient = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return nil, errors.New("agent init failed")
	}

	// Mock NewSessionFunc to avoid side effects (though it shouldn't be reached)
	origNewSessionFunc := NewSessionFunc
	defer func() { NewSessionFunc = origNewSessionFunc }()
	NewSessionFunc = func(d runner.DockerClient, a agent.Agent, workspace, image, project, provider, model string, maxAgents int) *runner.Session {
		return &runner.Session{}
	}

	cfg := SessionConfig{
		SessionName: "test-agent-fail",
		IsMock:      false,
		AllowDirty:  true,
		ProjectPath: t.TempDir(),
	}

	err := RunWorkflow(context.Background(), cfg)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to initialize agent")
}
