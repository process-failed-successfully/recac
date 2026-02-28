package main

import (
	"bytes"
	"context"
	"recac/internal/agent"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// MockCareerGitClient mocks IGitClient for career tests
type MockCareerGitClient struct {
	LogFunc func(repoPath string, args ...string) ([]string, error)
	// Other methods needed for interface satisfaction (can be stubs)
	CheckoutFunc          func(repoPath, commitOrBranch string) error
	DiffFunc              func(repoPath, commitA, commitB string) (string, error)
	DiffStagedFunc        func(repoPath string) (string, error)
	DiffStatFunc          func(repoPath, commitA, commitB string) (string, error)
	CurrentCommitSHAFunc  func(repoPath string) (string, error)
	RepoExistsFunc        func(repoPath string) bool
	CommitFunc            func(repoPath, message string) error
	FetchFunc             func(repoPath, remote, branch string) error
	CurrentBranchFunc     func(repoPath string) (string, error)
	CheckoutNewBranchFunc func(repoPath, branch string) error
	BisectStartFunc       func(repoPath, bad, good string) error
	BisectGoodFunc        func(repoPath, rev string) error
	BisectBadFunc         func(repoPath, rev string) error
	BisectResetFunc       func(repoPath string) error
	BisectLogFunc         func(repoPath string) ([]string, error)
	TagFunc               func(repoPath, version string) error
	DeleteTagFunc         func(repoPath, version string) error
	PushTagsFunc          func(repoPath string) error
	LatestTagFunc         func(repoPath string) (string, error)
	RunFunc               func(repoPath string, args ...string) (string, error)
	DeleteLocalBranchFunc func(repoPath, branch string) error
	CreatePRFunc          func(repoPath, title, body, base string) (string, error)
}

func (m *MockCareerGitClient) AbortMerge(directory string) error { return nil }
func (m *MockCareerGitClient) Recover(directory string) error { return nil }
func (m *MockCareerGitClient) Clean(directory string) error { return nil }
func (m *MockCareerGitClient) ResetHard(directory, remote, branch string) error { return nil }
func (m *MockCareerGitClient) StashPop(directory string) error { return nil }
func (m *MockCareerGitClient) DeleteRemoteBranch(directory, remote, branch string) error { return nil }
func (m *MockCareerGitClient) SetRemoteURL(directory, name, url string) error { return nil }
func (m *MockCareerGitClient) LocalBranchExists(directory, branch string) (bool, error) { return false, nil }
func (m *MockCareerGitClient) Config(directory, key, value string) error { return nil }
func (m *MockCareerGitClient) ConfigGlobal(key, value string) error { return nil }
func (m *MockCareerGitClient) ConfigAddGlobal(key, value string) error { return nil }
func (m *MockCareerGitClient) RemoteBranchExists(directory, remote, branch string) (bool, error) { return false, nil }
func (m *MockCareerGitClient) Clone(ctx context.Context, repoURL, directory string) error { return nil }
func (m *MockCareerGitClient) Push(directory, branch string) error { return nil }
func (m *MockCareerGitClient) Pull(directory, remote, branch string) error { return nil }
func (m *MockCareerGitClient) Stash(directory string) error { return nil }
func (m *MockCareerGitClient) Merge(directory, branchName string) error { return nil }

func (m *MockCareerGitClient) Log(repoPath string, args ...string) ([]string, error) {
	if m.LogFunc != nil {
		return m.LogFunc(repoPath, args...)
	}
	return nil, nil
}
func (m *MockCareerGitClient) Checkout(repoPath, commitOrBranch string) error { return nil }
func (m *MockCareerGitClient) Diff(repoPath, commitA, commitB string) (string, error) {
	return "", nil
}
func (m *MockCareerGitClient) DiffStaged(repoPath string) (string, error) { return "", nil }
func (m *MockCareerGitClient) DiffStat(repoPath, commitA, commitB string) (string, error) {
	return "", nil
}
func (m *MockCareerGitClient) CurrentCommitSHA(repoPath string) (string, error) { return "", nil }
func (m *MockCareerGitClient) RepoExists(repoPath string) bool                  { return true }
func (m *MockCareerGitClient) Commit(repoPath, message string) error            { return nil }
func (m *MockCareerGitClient) Fetch(repoPath, remote, branch string) error      { return nil }
func (m *MockCareerGitClient) CurrentBranch(repoPath string) (string, error)    { return "main", nil }
func (m *MockCareerGitClient) CheckoutNewBranch(repoPath, branch string) error  { return nil }
func (m *MockCareerGitClient) BisectStart(repoPath, bad, good string) error     { return nil }
func (m *MockCareerGitClient) BisectGood(repoPath, rev string) error            { return nil }
func (m *MockCareerGitClient) BisectBad(repoPath, rev string) error             { return nil }
func (m *MockCareerGitClient) BisectReset(repoPath string) error                { return nil }
func (m *MockCareerGitClient) BisectLog(repoPath string) ([]string, error)      { return nil, nil }
func (m *MockCareerGitClient) Tag(repoPath, version string) error               { return nil }
func (m *MockCareerGitClient) DeleteTag(repoPath, version string) error         { return nil }
func (m *MockCareerGitClient) PushTags(repoPath string) error                   { return nil }
func (m *MockCareerGitClient) LatestTag(repoPath string) (string, error)        { return "", nil }
func (m *MockCareerGitClient) Run(repoPath string, args ...string) (string, error) {
	if m.RunFunc != nil {
		return m.RunFunc(repoPath, args...)
	}
	return "", nil
}
func (m *MockCareerGitClient) DeleteLocalBranch(repoPath, branch string) error { return nil }
func (m *MockCareerGitClient) CreatePR(repoPath, title, body, base string) (string, error) {
	return "", nil
}

func (m *MockCareerGitClient) StashPush(directory, message string) error      { return nil }
func (m *MockCareerGitClient) StashList(directory string) ([]string, error)   { return nil, nil }
func (m *MockCareerGitClient) StashShow(directory, id string) (string, error) { return "", nil }
func (m *MockCareerGitClient) StashApply(directory, id string) error          { return nil }
func (m *MockCareerGitClient) StashDrop(directory, id string) error           { return nil }
func (m *MockCareerGitClient) StashClear(directory string) error              { return nil }
func (m *MockCareerGitClient) MergeBase(directory, ref1, ref2 string) (string, error) { return "", nil }
func (m *MockCareerGitClient) ResetSoft(directory, target string) error       { return nil }

// MockCareerAgent mocks agent.Agent
type MockCareerAgent struct {
	SendFunc       func(ctx context.Context, prompt string) (string, error)
	SendStreamFunc func(ctx context.Context, prompt string, onChunk func(string)) (string, error)
	CloseFunc      func() error
}

func (m *MockCareerAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.SendFunc != nil {
		return m.SendFunc(ctx, prompt)
	}
	return "", nil
}
func (m *MockCareerAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	if m.SendStreamFunc != nil {
		return m.SendStreamFunc(ctx, prompt, onChunk)
	}
	return "", nil
}

func TestCareerCmd(t *testing.T) {
	// Setup Mocks
	mockGit := &MockCareerGitClient{
		RunFunc: func(repoPath string, args ...string) (string, error) {
			if len(args) > 1 && args[0] == "config" && args[1] == "user.name" {
				return "Test User", nil
			}
			return "", nil
		},
		LogFunc: func(repoPath string, args ...string) ([]string, error) {
			// Check if name-only is requested
			for _, arg := range args {
				if arg == "--name-only" {
					return []string{"main.go", "pkg/feature.go", "Dockerfile"}, nil
				}
			}
			// Commits
			return []string{
				"hash1|2023-01-01|feat: added login",
				"hash2|2023-01-02|fix: resolved bug",
			}, nil
		},
	}

	mockAgent := &MockCareerAgent{
		SendFunc: func(ctx context.Context, prompt string) (string, error) {
			if strings.Contains(prompt, "Test User") && strings.Contains(prompt, "feat: added login") {
				return "## Achievements\n- Implemented login feature.", nil
			}
			return "Error: Prompt mismatch", nil
		},
	}

	// Override Factories
	origGitFactory := gitClientFactory
	gitClientFactory = func() IGitClient { return mockGit }
	defer func() { gitClientFactory = origGitFactory }()

	origAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = origAgentFactory }()

	// Capture Output
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)

	// Execute
	// We execute via rootCmd to ensure proper command parsing
	rootCmd.SetArgs([]string{"career", "--author", "Test User", "--since", "30d"})

	// Setup Viper
	viper.Set("provider", "mock")
	viper.Set("model", "mock-model")

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "## Achievements") {
		t.Errorf("Expected output to contain '## Achievements', got: %s", output)
	}
	if !strings.Contains(output, "Test User") {
		t.Errorf("Expected output to contain 'Test User', got: %s", output)
	}
}

func TestCareerCmd_NoCommits(t *testing.T) {
	mockGit := &MockCareerGitClient{
		RunFunc: func(repoPath string, args ...string) (string, error) {
			return "Test User", nil
		},
		LogFunc: func(repoPath string, args ...string) ([]string, error) {
			return []string{}, nil // No commits
		},
	}

	origGitFactory := gitClientFactory
	gitClientFactory = func() IGitClient { return mockGit }
	defer func() { gitClientFactory = origGitFactory }()

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"career", "--author", "Test User"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "No commits found") {
		t.Errorf("Expected 'No commits found', got: %s", output)
	}
}
