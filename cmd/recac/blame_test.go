package main

import (
	"context"
	"fmt"
	"os"
	"recac/internal/agent"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAgentForBlame is a mock implementation of agent.Agent
type MockAgentForBlame struct {
	mock.Mock
}

func (m *MockAgentForBlame) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *MockAgentForBlame) SendStream(ctx context.Context, prompt string, callback func(string)) (string, error) {
	args := m.Called(ctx, prompt, callback)
	callback(args.String(0))
	return args.String(0), args.Error(1)
}

func TestBlameCmd(t *testing.T) {
	// Setup Factories
	mockGit := &MockGitClient{}
	mockAgent := new(MockAgentForBlame)

	originalGitFactory := gitClientFactory
	originalAgentFactory := agentClientFactory
	defer func() {
		gitClientFactory = originalGitFactory
		agentClientFactory = originalAgentFactory
	}()

	gitClientFactory = func() IGitClient {
		return mockGit
	}
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// Create a temporary directory and file
	tmpDir, err := os.MkdirTemp("", "recac-blame-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	testFile := "main.go"
	testContent := `package main
import "fmt"
func main() {
	fmt.Println("Hello")
}
`
	err = os.WriteFile(testFile, []byte(testContent), 0644)
	assert.NoError(t, err)

	t.Run("Blame with line number", func(t *testing.T) {
		// Mock Git responses
		mockGit.RunFunc = func(repoPath string, args ...string) (string, error) {
			if args[0] == "blame" {
				// args: blame -L 3,3 --porcelain main.go
				assert.Equal(t, "3,3", args[2])
				return "abc1234 (Author Name 2023-01-01 3) fmt.Println(\"Hello\")", nil
			}
			if args[0] == "show" {
				assert.Equal(t, "abc1234", args[1])
				return "commit abc1234\nAuthor: Author Name\n\nFix output", nil
			}
			return "", fmt.Errorf("unexpected command: %v", args)
		}

		// Mock Agent response
		mockAgent.On("SendStream", mock.Anything, mock.MatchedBy(func(prompt string) bool {
			return strings.Contains(prompt, "line 3") &&
				strings.Contains(prompt, "abc1234") &&
				strings.Contains(prompt, "Fix output")
		}), mock.Anything).Return("Agent explanation", nil)

		// Run command
		dummyCmd := &cobra.Command{}
		args := []string{testFile, "3"}

		err := runBlame(dummyCmd, args)
		assert.NoError(t, err)

		mockAgent.AssertExpectations(t)
	})

	t.Run("Blame invalid line", func(t *testing.T) {
		args := []string{testFile, "not-a-number"}
		err := runBlame(&cobra.Command{}, args)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid line number")
	})

	t.Run("Blame file not found", func(t *testing.T) {
		args := []string{"non-existent.go", "1"}
		err := runBlame(&cobra.Command{}, args)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "file not found")
	})
}
