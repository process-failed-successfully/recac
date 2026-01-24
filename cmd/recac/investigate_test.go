package main

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"recac/internal/agent"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockAgentInvestigate struct {
	SendFunc func(ctx context.Context, prompt string) (string, error)
}

func (m *MockAgentInvestigate) Send(ctx context.Context, prompt string) (string, error) {
	if m.SendFunc != nil {
		return m.SendFunc(ctx, prompt)
	}
	return "", nil
}

func (m *MockAgentInvestigate) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return m.Send(ctx, prompt)
}

func TestInvestigate(t *testing.T) {
	// Save original factories
	origGit := gitClientFactory
	origAgent := agentClientFactory
	defer func() {
		gitClientFactory = origGit
		agentClientFactory = origAgent
	}()

	t.Run("No Commits Found", func(t *testing.T) {
		root, _, _ := newRootCmd()

		// Mock Git: Returns empty log
		gitClientFactory = func() IGitClient {
			return &MockGitClient{
				RepoExistsFunc: func(p string) bool { return true },
				LogFunc: func(p string, args ...string) ([]string, error) {
					return []string{}, nil
				},
			}
		}

		// Run
		// Need to reset global flags because they are shared
		investigateLimit = 5
		investigateSince = "1 day ago"
		investigatePath = ""

		output, err := executeCommand(root, "investigate", "slow login")
		require.NoError(t, err)
		assert.Contains(t, output, "No commits found")
	})

	t.Run("Finds Suspect Commit", func(t *testing.T) {
		root, _, _ := newRootCmd()

		// Mock Git: Returns 2 commits
		gitClientFactory = func() IGitClient {
			return &MockGitClient{
				RepoExistsFunc: func(p string) bool { return true },
				LogFunc: func(p string, args ...string) ([]string, error) {
					return []string{
						"12345678|dev1|fix login",
						"87654321|dev2|add feature",
					}, nil
				},
				DiffFunc: func(p, a, b string) (string, error) {
					return "diff content", nil
				},
			}
		}

		// Mock Agent: Returns high score for sha1
		agentClientFactory = func(ctx context.Context, p, m, d, n string) (agent.Agent, error) {
			return &MockAgentInvestigate{
				SendFunc: func(ctx context.Context, prompt string) (string, error) {
					if strings.Contains(prompt, "12345678") {
						return `{"score": 9, "reasoning": "This commit touches login logic."}`, nil
					}
					return `{"score": 2, "reasoning": "Unrelated."}`, nil
				},
			}, nil
		}

		investigateLimit = 5
		investigateSince = "1 day ago"

		output, err := executeCommand(root, "investigate", "slow login")
		require.NoError(t, err)

		assert.Contains(t, output, "Investigation Results:")
		assert.Contains(t, output, "9/10")
		assert.Contains(t, output, "1234567")
		assert.Contains(t, output, "Recommendation: Check commit 1234567")
	})

	t.Run("Handles Agent Error", func(t *testing.T) {
		root, _, _ := newRootCmd()

		// Mock Git: Returns 1 commit
		gitClientFactory = func() IGitClient {
			return &MockGitClient{
				RepoExistsFunc: func(p string) bool { return true },
				LogFunc: func(p string, args ...string) ([]string, error) {
					return []string{"12345678|dev1|msg"}, nil
				},
				DiffFunc: func(p, a, b string) (string, error) {
					return "diff", nil
				},
			}
		}

		// Mock Agent: Fails
		agentClientFactory = func(ctx context.Context, p, m, d, n string) (agent.Agent, error) {
			return &MockAgentInvestigate{
				SendFunc: func(ctx context.Context, prompt string) (string, error) {
					return "", fmt.Errorf("agent error")
				},
			}, nil
		}

		investigateLimit = 5
		investigateSince = "1 day ago"

		output, err := executeCommand(root, "investigate", "symptom")
		require.NoError(t, err)
		assert.Contains(t, output, "Agent failed: agent error")
	})
}
