package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"recac/internal/agent"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAgent is a mock implementation of the Agent interface.
// Re-declaring here or importing?
// test_helpers_test.go DOES NOT define MockAgent.
// changelog_test.go reused MockAgent from tickets_test.go.
// tickets_test.go seems to define it.
// To avoid redeclaration conflict if they are in the same package (main), I should check if it's already available.
// `go test` compiles all files in the package.
// If tickets_test.go defines MockAgent, it is available.
// I'll assume it is available. If not, I'll define ReleaseTestMockAgent.

type ReleaseTestMockAgent struct {
	mock.Mock
}

func (m *ReleaseTestMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *ReleaseTestMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	args := m.Called(ctx, prompt, onChunk)
	return args.String(0), args.Error(1)
}

// Ensure ReleaseTestMockAgent satisfies the interface
// The interface agent.Agent is not just Send/SendStream?
// Let's check agent/interface.go if possible.
// Factories.go says `agent.Agent`.
// I'll trust the previous tests.

func TestReleaseCmd(t *testing.T) {
	// Setup Mocks
	mockGit := &MockGitClient{}
	mockAgent := new(ReleaseTestMockAgent)

	// Override Factories
	oldGitFactory := gitClientFactory
	oldAgentFactory := agentClientFactory
	oldExecCommand := execCommand
	defer func() {
		gitClientFactory = oldGitFactory
		agentClientFactory = oldAgentFactory
		execCommand = oldExecCommand
	}()

	gitClientFactory = func() IGitClient {
		return mockGit
	}
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// Mock execCommand for "git describe"
	// We use the TestHelperProcess pattern or just a simpler mock if we can avoid real exec.
	// Since getLatestTag uses execCommand("git", ...).Output(), we need to intercept it.
	// Mocking exec.Command is hard without the HelperProcess pattern.
	// But `execCommand` is a variable. We can point it to a function that returns a *exec.Cmd
	// that runs our TestHelperProcess.

	execCommand = func(name string, args ...string) *exec.Cmd {
		// Intercept git describe and git log
		if name == "git" && len(args) > 0 {
			if args[0] == "describe" {
				cs := []string{"-test.run=TestHelperProcess", "--", "git-describe"}
				cmd := exec.Command(os.Args[0], cs...)
				cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
				return cmd
			}
			if args[0] == "log" && len(args) > 1 && len(args[1]) > 0 && args[1][0] == '-' {
				// This catches "git log --pretty=..." but args[1] is likely the range if present.
				// In getCommitLogs: args := []string{"log", fmt.Sprintf("--pretty=format:%%B%s", sep)}
				// if range present: args = append(args, commitRange)
				// So args[1] starts with --pretty.
				cs := []string{"-test.run=TestHelperProcess", "--", "git-log"}
				cmd := exec.Command(os.Args[0], cs...)

				// Determine type based on test case context?
				// Better to check an environment variable set by the test?
				// Or check if I can inspect the test name.
				// Simplest is to default to 'feat' and use env var in specific tests.
				logType := os.Getenv("TEST_GIT_LOG_TYPE")
				if logType == "" {
					logType = "feat"
				}

				cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1", "MOCK_GIT_LOG_TYPE=" + logType}
				return cmd
			}
		}
		return exec.Command(name, args...)
	}

	t.Run("Auto Bump Minor", func(t *testing.T) {
		os.Setenv("TEST_GIT_LOG_TYPE", "feat")
		defer os.Unsetenv("TEST_GIT_LOG_TYPE")

		// Used for Changelog generation
		mockGit.LogFunc = func(repoPath string, args ...string) ([]string, error) {
			return []string{"feat: new feature", "fix: bug"}, nil
		}
		mockGit.RepoExistsFunc = func(repoPath string) bool { return true }
		mockGit.TagFunc = func(repoPath, tag, message string) error {
			assert.Equal(t, "v1.1.0", tag)
			return nil
		}
		mockGit.CommitFunc = func(repoPath, message string) error {
			assert.Contains(t, message, "v1.1.0")
			return nil
		}
		mockGit.PushFunc = func(repoPath, branch string) error { return nil }
		mockGit.PushTagsFunc = func(repoPath string) error { return nil }
		mockGit.CurrentBranchFunc = func(repoPath string) (string, error) { return "main", nil }

		mockAgent.On("Send", mock.Anything, mock.Anything).Return("## v1.1.0\n- feat: new feature", nil)

		// Create dummy VERSION file
		tmpDir := t.TempDir()
		os.WriteFile(filepath.Join(tmpDir, "VERSION"), []byte("v1.0.0"), 0644)

		// Change CWD to temp dir for file updates
		oldWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(oldWd)

		// Execute
		output, err := executeCommand(rootCmd, "release")
		assert.NoError(t, err)
		assert.Contains(t, output, "Next version: v1.1.0 (minor - new feature detected)")

		// Verify file updated
		content, _ := os.ReadFile("VERSION")
		assert.Equal(t, "1.1.0", string(content))
	})

	t.Run("Breaking Change Bump", func(t *testing.T) {
		os.Setenv("TEST_GIT_LOG_TYPE", "breaking")
		defer os.Unsetenv("TEST_GIT_LOG_TYPE")

		// Used for Changelog generation
		mockGit.LogFunc = func(repoPath string, args ...string) ([]string, error) {
			return []string{"feat: regular feature", "fix: bug fix"}, nil
		}
		mockGit.RepoExistsFunc = func(repoPath string) bool { return true }
		mockGit.TagFunc = func(repoPath, tag, message string) error {
			assert.Equal(t, "v2.0.0", tag)
			return nil
		}
		mockGit.CommitFunc = func(repoPath, message string) error {
			assert.Contains(t, message, "v2.0.0")
			return nil
		}
		mockGit.PushFunc = func(repoPath, branch string) error { return nil }
		mockGit.PushTagsFunc = func(repoPath string) error { return nil }
		mockGit.CurrentBranchFunc = func(repoPath string) (string, error) { return "main", nil }

		mockAgent.On("Send", mock.Anything, mock.Anything).Return("## v2.0.0\n- BREAKING CHANGE", nil)

		// Create dummy VERSION file
		tmpDir := t.TempDir()
		os.WriteFile(filepath.Join(tmpDir, "VERSION"), []byte("v1.5.0"), 0644)

		oldWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(oldWd)

		output, err := executeCommand(rootCmd, "release")
		assert.NoError(t, err)
		assert.Contains(t, output, "Next version: v2.0.0 (major - breaking change detected)")
	})

	t.Run("Dry Run", func(t *testing.T) {
		os.Setenv("TEST_GIT_LOG_TYPE", "patch")
		defer os.Unsetenv("TEST_GIT_LOG_TYPE")

		mockGit.LogFunc = func(repoPath string, args ...string) ([]string, error) {
			return []string{"fix: bug"}, nil
		}
		mockGit.RepoExistsFunc = func(repoPath string) bool { return true }

		// Should NOT call Tag/Commit/Push
		mockGit.TagFunc = func(repoPath, tag, message string) error {
			t.Error("Tag should not be called in dry-run")
			return nil
		}

		// Helper process will return v1.0.0
		output, err := executeCommand(rootCmd, "release", "--dry-run")
		assert.NoError(t, err)
		assert.Contains(t, output, "Next version: v1.0.1 (patch - default)")
		assert.Contains(t, output, "Dry run enabled")
	})
}
