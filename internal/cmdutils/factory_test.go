package cmdutils

import (
	"context"
	"os"
	"recac/internal/git"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestGetJiraClient(t *testing.T) {
	// Setup
	viper.Reset()
	os.Unsetenv("JIRA_URL")
	os.Unsetenv("JIRA_USERNAME")
	os.Unsetenv("JIRA_API_TOKEN")

	t.Run("Missing Config", func(t *testing.T) {
		_, err := GetJiraClient(context.Background())
		assert.Error(t, err)

		viper.Set("jira.url", "https://example.atlassian.net")
		_, err = GetJiraClient(context.Background())
		assert.Error(t, err)

		viper.Set("jira.username", "user@example.com")
		_, err = GetJiraClient(context.Background())
		assert.Error(t, err)
	})

	t.Run("Valid Config", func(t *testing.T) {
		viper.Set("jira.url", "https://example.atlassian.net")
		viper.Set("jira.username", "user@example.com")
		viper.Set("jira.api_token", "token")

		client, err := GetJiraClient(context.Background())
		assert.NoError(t, err)
		assert.NotNil(t, client)
	})

	t.Run("Environment Variables", func(t *testing.T) {
		viper.Reset()
		os.Setenv("JIRA_URL", "https://env.atlassian.net")
		os.Setenv("JIRA_USERNAME", "env@example.com")
		os.Setenv("JIRA_API_TOKEN", "envtoken")
		defer func() {
			os.Unsetenv("JIRA_URL")
			os.Unsetenv("JIRA_USERNAME")
			os.Unsetenv("JIRA_API_TOKEN")
		}()

		client, err := GetJiraClient(context.Background())
		assert.NoError(t, err)
		assert.NotNil(t, client)
		// We can't easily check client internal state without reflection or exposure,
		// but NotError is good enough for factory.
	})
}

func TestGetAgentClient(t *testing.T) {
	viper.Reset()

	t.Run("Default Provider Gemini", func(t *testing.T) {
		viper.Set("api_key", "dummy")
		client, err := GetAgentClient(context.Background(), "", "", "", "")
		assert.NoError(t, err)
		assert.NotNil(t, client)
	})

	t.Run("Specific Provider", func(t *testing.T) {
		client, err := GetAgentClient(context.Background(), "openai", "gpt-4", "", "")
		assert.NoError(t, err)
		assert.NotNil(t, client)
	})

	t.Run("API Key from Environment", func(t *testing.T) {
		viper.Reset()
		os.Setenv("OPENAI_API_KEY", "env-key")
		defer os.Unsetenv("OPENAI_API_KEY")

		client, err := GetAgentClient(context.Background(), "openai", "", "", "")
		assert.NoError(t, err)
		assert.NotNil(t, client)
	})

	t.Run("Dummy Key Fallback", func(t *testing.T) {
		viper.Reset()
		client, err := GetAgentClient(context.Background(), "gemini", "", "", "")
		assert.NoError(t, err)
		assert.NotNil(t, client)
	})
}

type MockGitClient struct {
	repoExists          bool
	remoteBranchExists  bool
	cloneFn             func(ctx context.Context, repoURL, directory string) error
	checkoutFn          func(directory, branch string) error
	checkoutNewBranchFn func(directory, branch string) error
}

func (m *MockGitClient) Clone(ctx context.Context, repoURL, directory string) error {
	if m.cloneFn != nil {
		return m.cloneFn(ctx, repoURL, directory)
	}
	return nil
}

func (m *MockGitClient) RepoExists(directory string) bool {
	return m.repoExists
}

func (m *MockGitClient) Config(directory, key, value string) error {
	return nil
}

func (m *MockGitClient) ConfigAddGlobal(key, value string) error {
	return nil
}

func (m *MockGitClient) RemoteBranchExists(directory, remote, branch string) (bool, error) {
	return m.remoteBranchExists, nil
}

func (m *MockGitClient) Fetch(directory, remote, branch string) error {
	return nil
}

func (m *MockGitClient) Checkout(directory, branch string) error {
	if m.checkoutFn != nil {
		return m.checkoutFn(directory, branch)
	}
	return nil
}

func (m *MockGitClient) CheckoutNewBranch(directory, branch string) error {
	if m.checkoutNewBranchFn != nil {
		return m.checkoutNewBranchFn(directory, branch)
	}
	return nil
}

func (m *MockGitClient) Push(directory, branch string) error {
	return nil
}

func (m *MockGitClient) Pull(directory, remote, branch string) error {
	return nil
}

func (m *MockGitClient) DiffStat(workspace, startCommit, endCommit string) (string, error) {
	return "", nil
}

func (m *MockGitClient) CurrentCommitSHA(workspace string) (string, error) {
	return "", nil
}

func (m *MockGitClient) Stash(directory string) error {
	return nil
}

func (m *MockGitClient) Merge(directory, branchName string) error {
	return nil
}

func (m *MockGitClient) AbortMerge(directory string) error {
	return nil
}

func (m *MockGitClient) Recover(directory string) error {
	return nil
}

func (m *MockGitClient) Clean(directory string) error {
	return nil
}

func (m *MockGitClient) ResetHard(directory, remote, branch string) error {
	return nil
}

func (m *MockGitClient) StashPop(directory string) error {
	return nil
}

func (m *MockGitClient) DeleteRemoteBranch(directory, remote, branch string) error {
	return nil
}

func (m *MockGitClient) CurrentBranch(directory string) (string, error) {
	return "", nil
}

func (m *MockGitClient) Commit(directory, message string) error {
	return nil
}

func (m *MockGitClient) Diff(directory, startCommit, endCommit string) (string, error) {
	return "", nil
}

func (m *MockGitClient) DiffStaged(directory string) (string, error) {
	return "", nil
}

func (m *MockGitClient) SetRemoteURL(directory, name, url string) error {
	return nil
}

func (m *MockGitClient) DeleteLocalBranch(directory, branch string) error {
	return nil
}

func (m *MockGitClient) LocalBranchExists(directory, branch string) (bool, error) {
	return false, nil
}

func (m *MockGitClient) Log(directory string, args ...string) ([]string, error) {
	return []string{}, nil
}

func (m *MockGitClient) BisectStart(directory, bad, good string) error {
	return nil
}

func (m *MockGitClient) BisectGood(directory, rev string) error {
	return nil
}

func (m *MockGitClient) BisectBad(directory, rev string) error {
	return nil
}

func (m *MockGitClient) BisectReset(directory string) error {
	return nil
}

func (m *MockGitClient) BisectLog(directory string) ([]string, error) {
	return []string{}, nil
}

func (m *MockGitClient) Tag(directory, version string) error {
	return nil
}

func (m *MockGitClient) PushTags(directory string) error {
	return nil
}

func (m *MockGitClient) LatestTag(directory string) (string, error) {
	return "v0.0.0", nil
}

func TestSetupWorkspace(t *testing.T) {
	t.Run("Empty Repo URL", func(t *testing.T) {
		mockGitClient := &MockGitClient{}
		assert.Implements(t, (*git.IClient)(nil), mockGitClient)
		url, err := SetupWorkspace(context.Background(), mockGitClient, "", "/tmp/recac-test", "TEST-1", "", "")
		assert.NoError(t, err)
		assert.Equal(t, "", url)
	})

	t.Run("Clones when workspace does not exist", func(t *testing.T) {
		cloned := false
		mockGitClient := &MockGitClient{
			repoExists: false,
			cloneFn: func(ctx context.Context, repoURL, directory string) error {
				cloned = true
				return nil
			},
		}
		_, err := SetupWorkspace(context.Background(), mockGitClient, "https://github.com/example/repo", "/tmp/recac-test", "TEST-1", "", "")
		assert.NoError(t, err)
		assert.True(t, cloned)
	})

	t.Run("Skips clone when workspace exists", func(t *testing.T) {
		cloned := false
		mockGitClient := &MockGitClient{
			repoExists: true,
			cloneFn: func(ctx context.Context, repoURL, directory string) error {
				cloned = true
				return nil
			},
		}
		_, err := SetupWorkspace(context.Background(), mockGitClient, "https://github.com/example/repo", "/tmp/recac-test", "TEST-1", "", "")
		assert.NoError(t, err)
		assert.False(t, cloned)
	})

	t.Run("Checks out existing epic branch", func(t *testing.T) {
		checkedOut := ""
		mockGitClient := &MockGitClient{
			repoExists:         true,
			remoteBranchExists: true,
			checkoutFn: func(directory, branch string) error {
				if branch == "agent-epic/EPIC-1" {
					checkedOut = branch
				}
				return nil
			},
		}
		_, err := SetupWorkspace(context.Background(), mockGitClient, "https://github.com/example/repo", "/tmp/recac-test", "TEST-1", "EPIC-1", "")
		assert.NoError(t, err)
		assert.Equal(t, "agent-epic/EPIC-1", checkedOut)
	})

	t.Run("Creates new epic branch", func(t *testing.T) {
		newBranch := ""
		mockGitClient := &MockGitClient{
			repoExists:         true,
			remoteBranchExists: false,
			checkoutNewBranchFn: func(directory, branch string) error {
				if branch == "agent-epic/EPIC-1" {
					newBranch = branch
				}
				return nil
			},
		}
		_, err := SetupWorkspace(context.Background(), mockGitClient, "https://github.com/example/repo", "/tmp/recac-test", "TEST-1", "EPIC-1", "")
		assert.NoError(t, err)
		assert.Equal(t, "agent-epic/EPIC-1", newBranch)
	})

	t.Run("Creates unique feature branch", func(t *testing.T) {
		viper.Set("git.unique_branch_names", true)
		defer viper.Set("git.unique_branch_names", false)

		newBranch := ""
		mockGitClient := &MockGitClient{
			repoExists: true,
			checkoutNewBranchFn: func(directory, branch string) error {
				newBranch = branch
				return nil
			},
		}
		_, err := SetupWorkspace(context.Background(), mockGitClient, "https://github.com/example/repo", "/tmp/recac-test", "TEST-1", "", "20240101-120000")
		assert.NoError(t, err)
		assert.Equal(t, "agent/TEST-1-20240101-120000", newBranch)
	})

	t.Run("Creates stable feature branch", func(t *testing.T) {
		newBranch := ""
		mockGitClient := &MockGitClient{
			repoExists:         true,
			remoteBranchExists: false,
			checkoutNewBranchFn: func(directory, branch string) error {
				newBranch = branch
				return nil
			},
		}
		_, err := SetupWorkspace(context.Background(), mockGitClient, "https://github.com/example/repo", "/tmp/recac-test", "TEST-1", "", "")
		assert.NoError(t, err)
		assert.Equal(t, "agent/TEST-1", newBranch)
	})

	t.Run("Checks out existing stable feature branch", func(t *testing.T) {
		checkedOut := ""
		mockGitClient := &MockGitClient{
			repoExists:         true,
			remoteBranchExists: true,
			checkoutFn: func(directory, branch string) error {
				checkedOut = branch
				return nil
			},
		}
		_, err := SetupWorkspace(context.Background(), mockGitClient, "https://github.com/example/repo", "/tmp/recac-test", "TEST-1", "", "")
		assert.NoError(t, err)
		assert.Equal(t, "agent/TEST-1", checkedOut)
	})
}
