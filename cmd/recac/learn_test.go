package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"recac/internal/agent"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockLearnAgent struct {
	Response string
	Err      error
}

func (m *MockLearnAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.Err != nil {
		return "", m.Err
	}
	return m.Response, nil
}

func (m *MockLearnAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return m.Send(ctx, prompt)
}

type MockLearnGitClient struct {
	LogOutput []string
	Err       error
}

func (m *MockLearnGitClient) Log(repoPath string, args ...string) ([]string, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.LogOutput, nil
}

// Implement other interface methods as no-ops
func (m *MockLearnGitClient) RepoExists(repoPath string) bool { return true }
func (m *MockLearnGitClient) Checkout(repoPath, commitOrBranch string) error { return nil }
func (m *MockLearnGitClient) Diff(repoPath, commitA, commitB string) (string, error) { return "", nil }
func (m *MockLearnGitClient) DiffStaged(repoPath string) (string, error) { return "", nil }
func (m *MockLearnGitClient) DiffStat(repoPath, commitA, commitB string) (string, error) { return "", nil }
func (m *MockLearnGitClient) CurrentCommitSHA(repoPath string) (string, error) { return "sha", nil }
func (m *MockLearnGitClient) Commit(repoPath, message string) error { return nil }
func (m *MockLearnGitClient) Fetch(repoPath, remote, branch string) error { return nil }
func (m *MockLearnGitClient) CurrentBranch(repoPath string) (string, error) { return "main", nil }
func (m *MockLearnGitClient) CheckoutNewBranch(repoPath, branch string) error { return nil }
func (m *MockLearnGitClient) BisectStart(repoPath, bad, good string) error { return nil }
func (m *MockLearnGitClient) BisectGood(repoPath, rev string) error { return nil }
func (m *MockLearnGitClient) BisectBad(repoPath, rev string) error { return nil }
func (m *MockLearnGitClient) BisectReset(repoPath string) error { return nil }
func (m *MockLearnGitClient) BisectLog(repoPath string) ([]string, error) { return nil, nil }
func (m *MockLearnGitClient) Tag(repoPath, version string) error { return nil }
func (m *MockLearnGitClient) DeleteTag(repoPath, version string) error { return nil }
func (m *MockLearnGitClient) PushTags(repoPath string) error { return nil }
func (m *MockLearnGitClient) LatestTag(repoPath string) (string, error) { return "", nil }
func (m *MockLearnGitClient) Run(repoPath string, args ...string) (string, error) { return "", nil }
func (m *MockLearnGitClient) DeleteLocalBranch(repoPath, branch string) error { return nil }
func (m *MockLearnGitClient) CreatePR(repoPath, title, body, base string) (string, error) { return "", nil }
func (m *MockLearnGitClient) StashPush(d, msg string) error { return nil }
func (m *MockLearnGitClient) StashList(d string) ([]string, error) { return nil, nil }
func (m *MockLearnGitClient) StashShow(d, id string) (string, error) { return "", nil }
func (m *MockLearnGitClient) StashApply(d, id string) error { return nil }
func (m *MockLearnGitClient) StashDrop(d, id string) error { return nil }
func (m *MockLearnGitClient) StashClear(d string) error { return nil }
func (m *MockLearnGitClient) AbortMerge(dir string) error { return nil }
func (m *MockLearnGitClient) Recover(dir string) error { return nil }
func (m *MockLearnGitClient) Clean(dir string) error { return nil }
func (m *MockLearnGitClient) ResetHard(dir, remote, branch string) error { return nil }
func (m *MockLearnGitClient) StashPop(dir string) error { return nil }
func (m *MockLearnGitClient) DeleteRemoteBranch(dir, remote, branch string) error { return nil }
func (m *MockLearnGitClient) SetRemoteURL(dir, name, url string) error { return nil }
func (m *MockLearnGitClient) LocalBranchExists(dir, branch string) (bool, error) { return false, nil }
func (m *MockLearnGitClient) Config(dir, key, value string) error { return nil }
func (m *MockLearnGitClient) ConfigGlobal(key, value string) error { return nil }
func (m *MockLearnGitClient) ConfigAddGlobal(key, value string) error { return nil }
func (m *MockLearnGitClient) RemoteBranchExists(dir, remote, branch string) (bool, error) { return false, nil }
func (m *MockLearnGitClient) Clone(ctx context.Context, repoURL, dir string) error { return nil }
func (m *MockLearnGitClient) Push(dir, branch string) error { return nil }
func (m *MockLearnGitClient) Pull(dir, remote, branch string) error { return nil }
func (m *MockLearnGitClient) Stash(dir string) error { return nil }
func (m *MockLearnGitClient) Merge(dir, branchName string) error { return nil }

func TestRunLearn(t *testing.T) {
	// Setup temporary directory
	tmpDir, err := os.MkdirTemp("", "learn_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Change working directory
	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	// Create dummy files
	os.WriteFile("README.md", []byte("# Test Project"), 0644)
	os.WriteFile("main.go", []byte("package main"), 0644)

	// Override factories
	originalAgentFactory := agentClientFactory
	originalGitFactory := gitClientFactory
	originalContextFunc := generateContextFunc
	defer func() {
		agentClientFactory = originalAgentFactory
		gitClientFactory = originalGitFactory
		generateContextFunc = originalContextFunc
	}()

	mockAgent := &MockLearnAgent{
		Response: `{"context_summary": "# Context Summary", "system_prompt": "You are a test persona."}`,
	}
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	mockGit := &MockLearnGitClient{
		LogOutput: []string{"commit 1", "commit 2"},
	}
	gitClientFactory = func() IGitClient {
		return mockGit
	}

	generateContextFunc = func(opts ContextOptions) (string, error) {
		return "file tree", nil
	}

	// Create a temporary personas file
	personasFile := filepath.Join(tmpDir, ".recac", "personas.yaml")
	os.Setenv("RECAC_PERSONAS_FILE", personasFile)
	defer os.Unsetenv("RECAC_PERSONAS_FILE")

	// Set global flags manually since we are calling runLearn directly
	learnOutput = "context.md"
	learnPersona = "test-persona"
	learnLimit = 5

	// Create dummy command
	cmd := &cobra.Command{}
	cmd.SetOut(io.Discard)

	err = runLearn(cmd, []string{})
	require.NoError(t, err)

	// Verify output file
	content, err := os.ReadFile("context.md")
	require.NoError(t, err)
	assert.Equal(t, "# Context Summary", string(content))

	// Verify persona saved
	pm := agent.NewPersonaManager()
	err = pm.LoadPersonas()
	require.NoError(t, err)

	p, ok := pm.GetPersona("test-persona")
	assert.True(t, ok)
	assert.Equal(t, "You are a test persona.", p.SystemPrompt)
}
