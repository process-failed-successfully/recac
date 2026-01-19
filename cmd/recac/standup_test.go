package main

import (
	"context"
	"testing"
	"time"

	"recac/internal/agent"
	"recac/internal/runner"

	"github.com/stretchr/testify/assert"
)

// CapturingMockAgent implements agent.Agent and captures the prompt.
type CapturingMockAgent struct {
	LastPrompt string
	Response   string
}

func (m *CapturingMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	m.LastPrompt = prompt
	return m.Response, nil
}

func (m *CapturingMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	m.LastPrompt = prompt
	return m.Response, nil
}

func TestStandupCmd(t *testing.T) {
	// Setup Mocks
	mockGit := &MockGitClient{
		LogFunc: func(repoPath string, args ...string) ([]string, error) {
			// Mock git logs
			return []string{
				"abc1234|User|Fix bug in standup|2023-10-27T10:00:00Z",
				"def5678|User|Add feature X|2023-10-27T09:00:00Z",
			}, nil
		},
	}

	mockSM := NewMockSessionManager()
	mockSM.Sessions["session-1"] = &runner.SessionState{
		Name:      "session-1",
		Goal:      "Fix bug",
		Status:    "completed",
		StartTime: time.Now().Add(-2 * time.Hour),
		EndTime:   time.Now().Add(-1 * time.Hour),
	}
	mockSM.Sessions["session-2"] = &runner.SessionState{
		Name:      "session-2",
		Goal:      "Add feature",
		Status:    "failed",
		StartTime: time.Now().Add(-5 * time.Hour),
		EndTime:   time.Now().Add(-4 * time.Hour),
	}
	// Old session, should be ignored
	mockSM.Sessions["session-old"] = &runner.SessionState{
		Name:      "session-old",
		Goal:      "Ancient History",
		Status:    "completed",
		StartTime: time.Now().Add(-48 * time.Hour),
		EndTime:   time.Now().Add(-47 * time.Hour),
	}

	capturingAgent := &CapturingMockAgent{
		Response: "Mock Standup Report: Everything is great.",
	}

	// Override Factories
	oldGitFactory := gitClientFactory
	gitClientFactory = func() IGitClient {
		return mockGit
	}
	defer func() { gitClientFactory = oldGitFactory }()

	oldSMFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return mockSM, nil
	}
	defer func() { sessionManagerFactory = oldSMFactory }()

	oldAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return capturingAgent, nil
	}
	defer func() { agentClientFactory = oldAgentFactory }()

	// Execute
	output, err := executeCommand(rootCmd, "standup", "--since=24h")
	assert.NoError(t, err)

	// Verify Output
	assert.Contains(t, output, "Generating standup report")
	assert.Contains(t, output, "Mock Standup Report")

	// Verify Prompt Content (Strong Testing)
	prompt := capturingAgent.LastPrompt
	assert.Contains(t, prompt, "Generate a Daily Standup Report covering the last 24h")
	// Verify Git logs are included
	assert.Contains(t, prompt, "abc1234|User|Fix bug in standup")
	assert.Contains(t, prompt, "def5678|User|Add feature X")
	// Verify Session logs are included
	assert.Contains(t, prompt, "session-1: Fix bug (Status: completed)")
	assert.Contains(t, prompt, "session-2: Add feature (Status: failed)")
	// Verify Old session is NOT included
	assert.NotContains(t, prompt, "session-old")

	// Verify TODO stats
	// Since we are running in a test env, scanTodos might return nothing or real files if cwd is repo root.
	// But we expect "Codebase TODOs:" text.
	assert.Contains(t, prompt, "Codebase TODOs:")
}
