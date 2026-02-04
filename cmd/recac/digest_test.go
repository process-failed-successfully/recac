package main

import (
	"context"
	"fmt"
	"recac/internal/agent"
	"recac/internal/runner"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// DigestMockGitClient implements IGitClient for digest tests.
// It embeds MockGitClient to reuse existing mock functionality and adds no-op implementations
// for any other methods required by the interface but not used in these tests.
type DigestMockGitClient struct {
	MockGitClient
}

func (m *DigestMockGitClient) Clone(ctx context.Context, repoURL, directory string) error { return nil }
func (m *DigestMockGitClient) Config(directory, key, value string) error                  { return nil }
func (m *DigestMockGitClient) ConfigGlobal(key, value string) error                       { return nil }
func (m *DigestMockGitClient) ConfigAddGlobal(key, value string) error                    { return nil }
func (m *DigestMockGitClient) RemoteBranchExists(directory, remote, branch string) (bool, error) {
	return false, nil
}
func (m *DigestMockGitClient) Push(directory, branch string) error { return nil }
func (m *DigestMockGitClient) Pull(directory, remote, branch string) error {
	return nil
}
func (m *DigestMockGitClient) Stash(directory string) error             { return nil }
func (m *DigestMockGitClient) Merge(directory, branchName string) error { return nil }
func (m *DigestMockGitClient) AbortMerge(directory string) error        { return nil }
func (m *DigestMockGitClient) Recover(directory string) error           { return nil }
func (m *DigestMockGitClient) Clean(directory string) error             { return nil }
func (m *DigestMockGitClient) ResetHard(directory, remote, branch string) error {
	return nil
}
func (m *DigestMockGitClient) StashPop(directory string) error { return nil }
func (m *DigestMockGitClient) DeleteRemoteBranch(directory, remote, branch string) error {
	return nil
}
func (m *DigestMockGitClient) SetRemoteURL(directory, name, url string) error { return nil }
func (m *DigestMockGitClient) LocalBranchExists(directory, branch string) (bool, error) {
	return false, nil
}

// MockAgent for digest tests
type DigestMockAgent struct {
	SendFunc func(ctx context.Context, prompt string) (string, error)
}

func (m *DigestMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.SendFunc != nil {
		return m.SendFunc(ctx, prompt)
	}
	return "Mock Report", nil
}
func (m *DigestMockAgent) SendStream(ctx context.Context, prompt string, callback func(string)) (string, error) {
	return m.Send(ctx, prompt)
}
func (m *DigestMockAgent) Name() string { return "mock" }

func TestDigestCmd(t *testing.T) {
	// 1. Mock Git Client
	mockGit := &DigestMockGitClient{}
	mockGit.RepoExistsFunc = func(repoPath string) bool { return true }
	mockGit.LogFunc = func(repoPath string, args ...string) ([]string, error) {
		return []string{
			"a1b2c3d - Alice - Fix bug",
			"e5f6g7h - Bob - Add feature",
		}, nil
	}

	originalGitFactory := gitClientFactory
	gitClientFactory = func() IGitClient {
		return mockGit
	}
	defer func() { gitClientFactory = originalGitFactory }()

	// 2. Mock Session Manager
	mockSM := NewMockSessionManager()
	mockSM.Sessions["session-1"] = &runner.SessionState{
		Name:      "session-1",
		Status:    "completed",
		StartTime: time.Now().Add(-1 * time.Hour),
	}

	originalSMFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return mockSM, nil
	}
	defer func() { sessionManagerFactory = originalSMFactory }()

	// 3. Mock Agent
	mockAgent := &DigestMockAgent{
		SendFunc: func(ctx context.Context, prompt string) (string, error) {
			// Verify prompt contains expected data
			if !strings.Contains(prompt, "Fix bug") {
				return "", fmt.Errorf("missing git log in prompt")
			}
			if !strings.Contains(prompt, "session-1") {
				return "", fmt.Errorf("missing session in prompt")
			}
			return "# Daily Digest\nEverything is awesome.", nil
		},
	}

	originalAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalAgentFactory }()

	// 4. Mock Audit
	originalRunAudit := runAuditFunc
	runAuditFunc = func(root string) (*AuditResult, error) {
		return &AuditResult{
			Score: 95,
			Complexity: ComplexityStats{
				Average: 5.5,
			},
			Duplication: DuplicationStats{
				Blocks: 0,
			},
			Todos: TodoStats{
				Count: 2,
			},
		}, nil
	}
	defer func() { runAuditFunc = originalRunAudit }()

	// 5. Run Command
	// We use the shared executeCommand helper which sets up flags, output buffers, etc.
	// We run "digest" subcommand on rootCmd.
	output, err := executeCommand(rootCmd, "digest")
	assert.NoError(t, err)

	assert.Contains(t, output, "# Daily Digest")
	assert.Contains(t, output, "Everything is awesome")
}
