package workflow

import (
	"context"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"recac/internal/cmdutils"
	"recac/internal/runner"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunWorkflow_MaxTokens_Propagation(t *testing.T) {
	// 1. Setup Mock for GetAgentClient
	originalGetAgentClient := cmdutils.GetAgentClient
	defer func() { cmdutils.GetAgentClient = originalGetAgentClient }()

	cmdutils.GetAgentClient = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return agent.NewMockAgent(), nil
	}

	// 2. Setup Mock for NewSessionFunc to capture the session
	var capturedSession *runner.Session
	originalNewSessionFunc := NewSessionFunc
	defer func() { NewSessionFunc = originalNewSessionFunc }()

	NewSessionFunc = func(client runner.DockerClient, a agent.Agent, workDir, image, project, provider, model string, maxAgents int) *runner.Session {
		// Call original to get a valid session object structure
		s := originalNewSessionFunc(client, a, workDir, image, project, provider, model, maxAgents)
		capturedSession = s
		return s
	}

	// Create temp workspace
	tempDir, err := os.MkdirTemp("", "test-workflow-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Ensure app_spec.txt exists (RunLoop check)
	err = os.WriteFile(filepath.Join(tempDir, "app_spec.txt"), []byte("test spec"), 0644)
	assert.NoError(t, err)

	ctx := context.Background()
	cfg := SessionConfig{
		SessionName: "test-tokens",
		IsMock:      false, // Must be false to trigger InitializeAgentState
		MaxTokens:   99999, // Specific value to test
		ProjectPath: tempDir,
		MaxIterations: 0, // Should cause RunLoop to exit immediately
		AllowDirty: true, // skip git check
	}

	// Run Workflow
	// It might return error due to MaxIterations=0 (ErrMaxIterations) or other things (Docker fail)
	// We expect "reached max iterations" error or nil depending on implementation.
	// But crucial part is InitializeAgentState runs before RunLoop.

	_ = RunWorkflow(ctx, cfg)

	// Verify captured session
	assert.NotNil(t, capturedSession)

	// Verify StateManager has correct MaxTokens
	// We load the state from the file created/updated by InitializeAgentState
	// The file is in tempDir/.agent_state.json

	statePath := filepath.Join(tempDir, ".agent_state.json")
	state, err := agent.LoadState(statePath)
	assert.NoError(t, err)
	assert.Equal(t, 99999, state.MaxTokens)
}
