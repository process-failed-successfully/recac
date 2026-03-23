package main

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"recac/internal/agent" // Import agent package
)

// MockGitClient for blog tests
type MockBlogGitClient struct {
	mock.Mock
}

func (m *MockBlogGitClient) Log(dir string, args ...string) ([]string, error) {
	called := m.Called(dir, args)
	return called.Get(0).([]string), called.Error(1)
}

func (m *MockBlogGitClient) RepoExists(dir string) bool {
	args := m.Called(dir)
	return args.Bool(0)
}

// Stubs for other interface methods
func (m *MockBlogGitClient) DiffStaged(dir string) (string, error) { return "", nil }
func (m *MockBlogGitClient) Commit(dir, msg string) error          { return nil }
func (m *MockBlogGitClient) Push(dir, branch string) error                 { return nil }
func (m *MockBlogGitClient) Pull(dir, remote, branch string) error                 { return nil }
func (m *MockBlogGitClient) Status(dir string) (string, error)     { return "", nil }
func (m *MockBlogGitClient) Init(dir string) error                 { return nil }
func (m *MockBlogGitClient) Add(dir string, args ...string) error  { return nil }
func (m *MockBlogGitClient) Diff(dir, startCommit, endCommit string) (string, error) { return "", nil }
func (m *MockBlogGitClient) Checkout(dir, branch string) error     { return nil }
func (m *MockBlogGitClient) CheckoutNew(dir, branch string) error  { return nil }
func (m *MockBlogGitClient) CheckoutNewBranch(dir, branch string) error  { return nil }
func (m *MockBlogGitClient) CurrentBranch(dir string) (string, error) { return "main", nil }
func (m *MockBlogGitClient) Stash(dir string) error                { return nil }
func (m *MockBlogGitClient) StashPop(dir string) error             { return nil }
func (m *MockBlogGitClient) MergeBase(directory, ref1, ref2 string) (string, error) { return "", nil }
func (m *MockBlogGitClient) ResetSoft(directory, target string) error       { return nil }
func (m *MockBlogGitClient) StashPush(dir string, msg string) error { return nil }
func (m *MockBlogGitClient) StashList(dir string) ([]string, error) { return []string{}, nil }
func (m *MockBlogGitClient) StashShow(dir string, index string) (string, error) { return "", nil }
func (m *MockBlogGitClient) StashApply(dir string, index string) error { return nil }
func (m *MockBlogGitClient) StashDrop(dir string, index string) error { return nil }
func (m *MockBlogGitClient) StashClear(dir string) error { return nil }
func (m *MockBlogGitClient) Fetch(dir, remote, branch string) error { return nil }
func (m *MockBlogGitClient) LatestTag(dir string) (string, error) { return "", nil }
func (m *MockBlogGitClient) Tag(dir, version string) error { return nil }
func (m *MockBlogGitClient) PushTags(dir string) error { return nil }
func (m *MockBlogGitClient) Run(dir string, args ...string) (string, error) { return "", nil }
func (m *MockBlogGitClient) BisectStart(dir, bad, good string) error { return nil }
func (m *MockBlogGitClient) BisectGood(dir, rev string) error { return nil }
func (m *MockBlogGitClient) BisectBad(dir, rev string) error { return nil }
func (m *MockBlogGitClient) BisectReset(dir string) error { return nil }
func (m *MockBlogGitClient) BisectLog(dir string) ([]string, error) { return []string{}, nil }
func (m *MockBlogGitClient) CreatePR(dir, title, body, base string) (string, error) { return "", nil }
func (m *MockBlogGitClient) DeleteTag(dir, version string) error { return nil }
func (m *MockBlogGitClient) DeleteLocalBranch(dir, branch string) error { return nil }
func (m *MockBlogGitClient) DeleteRemoteBranch(dir, remote, branch string) error { return nil }
func (m *MockBlogGitClient) DiffStat(dir, startCommit, endCommit string) (string, error) { return "", nil }
func (m *MockBlogGitClient) CurrentCommitSHA(dir string) (string, error) { return "", nil }
func (m *MockBlogGitClient) AbortMerge(dir string) error { return nil }
func (m *MockBlogGitClient) Recover(dir string) error { return nil }
func (m *MockBlogGitClient) Clean(dir string) error { return nil }
func (m *MockBlogGitClient) ResetHard(dir, remote, branch string) error { return nil }
func (m *MockBlogGitClient) SetRemoteURL(dir, name, url string) error { return nil }
func (m *MockBlogGitClient) LocalBranchExists(dir, branch string) (bool, error) { return false, nil }
func (m *MockBlogGitClient) Config(dir, key, value string) error { return nil }
func (m *MockBlogGitClient) ConfigGlobal(key, value string) error { return nil }
func (m *MockBlogGitClient) ConfigAddGlobal(key, value string) error { return nil }
func (m *MockBlogGitClient) RemoteBranchExists(dir, remote, branch string) (bool, error) { return false, nil }
func (m *MockBlogGitClient) Clone(ctx context.Context, repoURL, dir string) error { return nil }
func (m *MockBlogGitClient) Merge(dir, branchName string) error { return nil }

// MockAgent for blog tests
type MockBlogAgent struct {
	mock.Mock
}

func (m *MockBlogAgent) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *MockBlogAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return m.Send(ctx, prompt)
}

func TestBlogCommand(t *testing.T) {
	// Setup
	mockGit := new(MockBlogGitClient)
	mockAgent := new(MockBlogAgent)

	// Inject Mocks
	originalGitFactory := gitClientFactory
	gitClientFactory = func() IGitClient {
		return mockGit
	}
	defer func() { gitClientFactory = originalGitFactory }()

	originalAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, p, m, pp, pn string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalAgentFactory }()

	// Temp output file
	tmpFile, err := os.CreateTemp("", "blog_post_*.md")
	assert.NoError(t, err)
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	// Mock Expectations
	cwd, _ := os.Getwd()
	mockGit.On("RepoExists", cwd).Return(true)
	// Expect Log call with loose match on arguments
	mockGit.On("Log", cwd, mock.Anything).Return([]string{"hash|user|feat: cool feature", "hash|user|fix: bug"}, nil)

	mockAgent.On("Send", mock.Anything, mock.Anything).Return("# New Blog Post\n\nCheck out the new features!", nil)

	// Execute Command via rootCmd to ensure proper context
	// Reset flags (vars) to defaults is still good practice if shared
	blogSince = "1 week ago"
	blogOutput = "blog_post.md"
	blogStyle = "announcement"
	blogFiles = nil

	rootCmd.SetArgs([]string{
		"blog",
		"--output", tmpPath,
		"--style", "announcement",
		"--since", "2 days ago",
	})

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)

	// Execute
	err = rootCmd.Execute()
	assert.NoError(t, err)

	// Verify Output
	assert.Contains(t, buf.String(), "Drafting blog post")
	assert.Contains(t, buf.String(), "Blog post saved to")

	// Verify File Content
	content, err := os.ReadFile(tmpPath)
	assert.NoError(t, err)
	assert.Equal(t, "# New Blog Post\n\nCheck out the new features!", string(content))

	mockGit.AssertExpectations(t)
	mockAgent.AssertExpectations(t)
}

func TestBlogCommand_NoCommits(t *testing.T) {
	// Setup
	mockGit := new(MockBlogGitClient)
	mockAgent := new(MockBlogAgent)

	originalGitFactory := gitClientFactory
	gitClientFactory = func() IGitClient { return mockGit }
	defer func() { gitClientFactory = originalGitFactory }()

	originalAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, p, m, pp, pn string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalAgentFactory }()

	cwd, _ := os.Getwd()
	mockGit.On("RepoExists", cwd).Return(true)
	mockGit.On("Log", cwd, mock.Anything).Return([]string{}, nil) // No commits

	blogSince = "1 week ago"

	var buf bytes.Buffer
	rootCmd.SetArgs([]string{"blog"})
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)

	err := rootCmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "No commits found since")
}

func TestBlogCommand_NotGitRepo(t *testing.T) {
	mockGit := new(MockBlogGitClient)

	originalGitFactory := gitClientFactory
	gitClientFactory = func() IGitClient { return mockGit }
	defer func() { gitClientFactory = originalGitFactory }()

	cwd, _ := os.Getwd()
	mockGit.On("RepoExists", cwd).Return(false) // Not a repo

	var buf bytes.Buffer
	rootCmd.SetArgs([]string{"blog"})
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)

	err := rootCmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a git repository")
}
