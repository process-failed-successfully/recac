package main

import (
	"bytes"
	"context"
	"recac/internal/agent"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

// MockStashAgent
type MockStashAgent struct {
	Response   string
	LastPrompt string
}

func (m *MockStashAgent) Send(ctx context.Context, prompt string) (string, error) {
	m.LastPrompt = prompt
	return m.Response, nil
}

func (m *MockStashAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return m.Response, nil
}

func TestStashSave_WithAI(t *testing.T) {
	// Setup Mocks
	mockGit := &MockGitClient{
		DiffStagedFunc: func(repoPath string) (string, error) {
			return "diff --git a/file.go b/file.go\n+new content", nil
		},
		StashPushFunc: func(dir, message string) error {
			assert.Contains(t, message, "AI Generated Message")
			return nil
		},
		RepoExistsFunc: func(path string) bool { return true },
	}

	mockAgent := &MockStashAgent{
		Response: "AI Generated Message",
	}

	// Override Factories
	oldGitFactory := gitClientFactory
	gitClientFactory = func() IGitClient { return mockGit }
	defer func() { gitClientFactory = oldGitFactory }()

	oldAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, p, m, path, name string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = oldAgentFactory }()

	// Execute
	cmd := &cobra.Command{}
	cmd.SetOut(new(bytes.Buffer))
	err := runStashSave(cmd, []string{}) // No message provided
	assert.NoError(t, err)
	assert.Contains(t, mockAgent.LastPrompt, "new content")
}

func TestStashList_Analyze(t *testing.T) {
	// Setup
	mockGit := &MockGitClient{
		StashListFunc: func(dir string) ([]string, error) {
			return []string{"stash@{0}: WIP on main: 12345 message"}, nil
		},
		StashShowFunc: func(dir, id string) (string, error) {
			assert.Equal(t, "stash@{0}", id)
			return "diff content", nil
		},
	}

	mockAgent := &MockStashAgent{
		Response: "Refactored login",
	}

	// Override
	oldGitFactory := gitClientFactory
	gitClientFactory = func() IGitClient { return mockGit }
	defer func() { gitClientFactory = oldGitFactory }()

	oldAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, p, m, path, name string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = oldAgentFactory }()

	// Set Flag
	stashAnalyze = true
	defer func() { stashAnalyze = false }()

	// Execute
	buf := new(bytes.Buffer)
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	err := runStashList(cmd, []string{})

	assert.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "stash@{0}: Refactored login")
}
