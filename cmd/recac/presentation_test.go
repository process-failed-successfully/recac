package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"recac/internal/agent"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockAgentForPresentation struct {
	SendFunc func(ctx context.Context, prompt string) (string, error)
}

func (m *MockAgentForPresentation) Send(ctx context.Context, prompt string) (string, error) {
	if m.SendFunc != nil {
		return m.SendFunc(ctx, prompt)
	}
	return "", nil
}

func (m *MockAgentForPresentation) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return m.Send(ctx, prompt)
}

func TestPresentation(t *testing.T) {
	// Save original factories
	origGit := gitClientFactory
	origAgent := agentClientFactory
	defer func() {
		gitClientFactory = origGit
		agentClientFactory = origAgent
	}()

	t.Run("Generate Presentation Successfully", func(t *testing.T) {
		root, _, _ := newRootCmd()
		outFile := "test_presentation.md"
		defer os.Remove(outFile)

		// Mock Git
		gitClientFactory = func() IGitClient {
			return &MockGitClient{
				RepoExistsFunc: func(p string) bool { return true },
				LogFunc: func(p string, args ...string) ([]string, error) {
					// Check if we are asking for body or log list
					// args usually: --pretty=format:%H|%an|%s -n10 ...
					for _, arg := range args {
						if strings.Contains(arg, "%b") {
							// asking for body
							return []string{"Detailed commit body..."}, nil
						}
					}
					return []string{
						"11111111|Alice|feat: Add login",
						"22222222|Bob|fix: Crash on startup",
					}, nil
				},
			}
		}

		// Mock Agent
		agentClientFactory = func(ctx context.Context, p, m, d, n string) (agent.Agent, error) {
			return &MockAgentForPresentation{
				SendFunc: func(ctx context.Context, prompt string) (string, error) {
					assert.Contains(t, prompt, "Alice")
					assert.Contains(t, prompt, "Add login")
					assert.Contains(t, prompt, "Bob")
					return "# Title Slide\n\n- Slide 1\n- Slide 2", nil
				},
			}, nil
		}

		// Execute
		// presentation --output test_presentation.md
		output, err := executeCommand(root, "presentation", "--output", outFile)
		require.NoError(t, err)

		assert.Contains(t, output, "Presentation saved to")

		// Verify File
		content, err := os.ReadFile(outFile)
		require.NoError(t, err)

		strContent := string(content)
		assert.Contains(t, strContent, "marp: true")
		assert.Contains(t, strContent, "# Title Slide")
	})

	t.Run("No Commits Found", func(t *testing.T) {
		root, _, _ := newRootCmd()

		// Mock Git: Empty logs
		gitClientFactory = func() IGitClient {
			return &MockGitClient{
				RepoExistsFunc: func(p string) bool { return true },
				LogFunc: func(p string, args ...string) ([]string, error) {
					return []string{}, nil
				},
			}
		}

		output, err := executeCommand(root, "presentation", "--since", "1 year ago")
		require.NoError(t, err)
		assert.Contains(t, output, "No commits found")
	})

	t.Run("Agent Failure", func(t *testing.T) {
		root, _, _ := newRootCmd()
		outFile := "fail_pres.md"
		defer os.Remove(outFile)

		gitClientFactory = func() IGitClient {
			return &MockGitClient{
				RepoExistsFunc: func(p string) bool { return true },
				LogFunc: func(p string, args ...string) ([]string, error) {
					return []string{"12345678|User|Msg"}, nil
				},
			}
		}

		agentClientFactory = func(ctx context.Context, p, m, d, n string) (agent.Agent, error) {
			return &MockAgentForPresentation{
				SendFunc: func(ctx context.Context, prompt string) (string, error) {
					return "", fmt.Errorf("AI error")
				},
			}, nil
		}

		output, err := executeCommand(root, "presentation", "--output", outFile)
		require.Error(t, err) // Should error out
		// Since SilenceErrors: true, the error is returned but not printed to output
		assert.Contains(t, err.Error(), "agent failed")
		assert.Contains(t, output, "Generating slides...")
	})
}
