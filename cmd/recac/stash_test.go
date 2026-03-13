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

func TestRunStashAction(t *testing.T) {
	mockGit := &MockGitClient{
		StashApplyFunc: func(dir, id string) error {
			assert.Equal(t, "stash@{0}", id)
			return nil
		},
		StashDropFunc: func(dir, id string) error {
			assert.Equal(t, "stash@{1}", id)
			return nil
		},
		StashShowFunc: func(dir, id string) (string, error) {
			assert.Equal(t, "stash@{2}", id)
			return "stash output", nil
		},
		RunFunc: func(dir string, args ...string) (string, error) {
			assert.Equal(t, []string{"stash", "pop", "stash@{3}"}, args)
			return "popped", nil
		},
	}

	oldGitFactory := gitClientFactory
	gitClientFactory = func() IGitClient { return mockGit }
	defer func() { gitClientFactory = oldGitFactory }()

	cmd := &cobra.Command{}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err := runStashAction(cmd, []string{"stash@{0}"}, "apply")
	assert.NoError(t, err)

	err = runStashAction(cmd, []string{"stash@{1}"}, "drop")
	assert.NoError(t, err)

	err = runStashAction(cmd, []string{"stash@{2}"}, "show")
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "stash output")

	err = runStashAction(cmd, []string{"stash@{3}"}, "pop")
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "popped")
}

func TestRunStashClear(t *testing.T) {
	mockGit := &MockGitClient{
		StashClearFunc: func(dir string) error {
			return nil
		},
	}

	oldGitFactory := gitClientFactory
	gitClientFactory = func() IGitClient { return mockGit }
	defer func() { gitClientFactory = oldGitFactory }()

	cmd := &cobra.Command{}
	err := runStashClear(cmd, []string{})
	assert.NoError(t, err)
}
