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


func TestSetupWorkspace(t *testing.T) {
	t.Run("Empty Repo URL", func(t *testing.T) {
		mockGitClient := &git.MockGitClient{}
		assert.Implements(t, (*git.IClient)(nil), mockGitClient)
		url, err := SetupWorkspace(context.Background(), mockGitClient, "", "/tmp/recac-test", "TEST-1", "", "")
		assert.NoError(t, err)
		assert.Equal(t, "", url)
	})

	t.Run("Clones when workspace does not exist", func(t *testing.T) {
		mockGitClient := &git.MockGitClient{}
		mockGitClient.On("RepoExists", "/tmp/recac-test").Return(false)
		mockGitClient.On("Clone", context.Background(), "https://github.com/example/repo", "/tmp/recac-test").Return(nil)
		mockGitClient.On("ConfigAddGlobal", "safe.directory", "/tmp/recac-test").Return(nil)
		mockGitClient.On("Config", "/tmp/recac-test", "user.email", "agent@recac.com").Return(nil)
		mockGitClient.On("Config", "/tmp/recac-test", "user.name", "Recac Agent").Return(nil)
		mockGitClient.On("RemoteBranchExists", "/tmp/recac-test", "origin", "agent/TEST-1").Return(false, nil)
		mockGitClient.On("CheckoutNewBranch", "/tmp/recac-test", "agent/TEST-1").Return(nil)
		mockGitClient.On("Push", "/tmp/recac-test", "agent/TEST-1").Return(nil)
		_, err := SetupWorkspace(context.Background(), mockGitClient, "https://github.com/example/repo", "/tmp/recac-test", "TEST-1", "", "")
		assert.NoError(t, err)
		mockGitClient.AssertExpectations(t)
	})

	t.Run("Skips clone when workspace exists", func(t *testing.T) {
		mockGitClient := &git.MockGitClient{}
		mockGitClient.On("RepoExists", "/tmp/recac-test").Return(true)
		mockGitClient.On("ConfigAddGlobal", "safe.directory", "/tmp/recac-test").Return(nil)
		mockGitClient.On("Config", "/tmp/recac-test", "user.email", "agent@recac.com").Return(nil)
		mockGitClient.On("Config", "/tmp/recac-test", "user.name", "Recac Agent").Return(nil)
		mockGitClient.On("RemoteBranchExists", "/tmp/recac-test", "origin", "agent/TEST-1").Return(false, nil)
		mockGitClient.On("CheckoutNewBranch", "/tmp/recac-test", "agent/TEST-1").Return(nil)
		mockGitClient.On("Push", "/tmp/recac-test", "agent/TEST-1").Return(nil)
		_, err := SetupWorkspace(context.Background(), mockGitClient, "https://github.com/example/repo", "/tmp/recac-test", "TEST-1", "", "")
		assert.NoError(t, err)
		mockGitClient.AssertNotCalled(t, "Clone")
	})

	t.Run("Checks out existing epic branch", func(t *testing.T) {
		mockGitClient := &git.MockGitClient{}
		mockGitClient.On("RepoExists", "/tmp/recac-test").Return(true)
		mockGitClient.On("ConfigAddGlobal", "safe.directory", "/tmp/recac-test").Return(nil)
		mockGitClient.On("Config", "/tmp/recac-test", "user.email", "agent@recac.com").Return(nil)
		mockGitClient.On("Config", "/tmp/recac-test", "user.name", "Recac Agent").Return(nil)
		mockGitClient.On("RemoteBranchExists", "/tmp/recac-test", "origin", "agent-epic/EPIC-1").Return(true, nil)
		mockGitClient.On("Fetch", "/tmp/recac-test", "origin", "agent-epic/EPIC-1").Return(nil)
		mockGitClient.On("Checkout", "/tmp/recac-test", "agent-epic/EPIC-1").Return(nil)
		mockGitClient.On("RemoteBranchExists", "/tmp/recac-test", "origin", "agent/TEST-1").Return(false, nil)
		mockGitClient.On("CheckoutNewBranch", "/tmp/recac-test", "agent/TEST-1").Return(nil)
		mockGitClient.On("Push", "/tmp/recac-test", "agent/TEST-1").Return(nil)
		_, err := SetupWorkspace(context.Background(), mockGitClient, "https://github.com/example/repo", "/tmp/recac-test", "TEST-1", "EPIC-1", "")
		assert.NoError(t, err)
		mockGitClient.AssertExpectations(t)
	})

	t.Run("Creates new epic branch", func(t *testing.T) {
		mockGitClient := &git.MockGitClient{}
		mockGitClient.On("RepoExists", "/tmp/recac-test").Return(true)
		mockGitClient.On("ConfigAddGlobal", "safe.directory", "/tmp/recac-test").Return(nil)
		mockGitClient.On("Config", "/tmp/recac-test", "user.email", "agent@recac.com").Return(nil)
		mockGitClient.On("Config", "/tmp/recac-test", "user.name", "Recac Agent").Return(nil)
		mockGitClient.On("RemoteBranchExists", "/tmp/recac-test", "origin", "agent-epic/EPIC-1").Return(false, nil)
		mockGitClient.On("CheckoutNewBranch", "/tmp/recac-test", "agent-epic/EPIC-1").Return(nil)
		mockGitClient.On("Push", "/tmp/recac-test", "agent-epic/EPIC-1").Return(nil)
		mockGitClient.On("RemoteBranchExists", "/tmp/recac-test", "origin", "agent/TEST-1").Return(false, nil)
		mockGitClient.On("CheckoutNewBranch", "/tmp/recac-test", "agent/TEST-1").Return(nil)
		mockGitClient.On("Push", "/tmp/recac-test", "agent/TEST-1").Return(nil)
		_, err := SetupWorkspace(context.Background(), mockGitClient, "https://github.com/example/repo", "/tmp/recac-test", "TEST-1", "EPIC-1", "")
		assert.NoError(t, err)
		mockGitClient.AssertExpectations(t)
	})

	t.Run("Creates unique feature branch", func(t *testing.T) {
		viper.Set("git.unique_branch_names", true)
		defer viper.Set("git.unique_branch_names", false)

		mockGitClient := &git.MockGitClient{}
		mockGitClient.On("RepoExists", "/tmp/recac-test").Return(true)
		mockGitClient.On("ConfigAddGlobal", "safe.directory", "/tmp/recac-test").Return(nil)
		mockGitClient.On("Config", "/tmp/recac-test", "user.email", "agent@recac.com").Return(nil)
		mockGitClient.On("Config", "/tmp/recac-test", "user.name", "Recac Agent").Return(nil)
		mockGitClient.On("RemoteBranchExists", "/tmp/recac-test", "origin", "agent/TEST-1-20240101-120000").Return(false, nil)
		mockGitClient.On("CheckoutNewBranch", "/tmp/recac-test", "agent/TEST-1-20240101-120000").Return(nil)
		mockGitClient.On("Push", "/tmp/recac-test", "agent/TEST-1-20240101-120000").Return(nil)
		_, err := SetupWorkspace(context.Background(), mockGitClient, "https://github.com/example/repo", "/tmp/recac-test", "TEST-1", "", "20240101-120000")
		assert.NoError(t, err)
		mockGitClient.AssertExpectations(t)
	})

	t.Run("Creates stable feature branch", func(t *testing.T) {
		mockGitClient := &git.MockGitClient{}
		mockGitClient.On("RepoExists", "/tmp/recac-test").Return(true)
		mockGitClient.On("ConfigAddGlobal", "safe.directory", "/tmp/recac-test").Return(nil)
		mockGitClient.On("Config", "/tmp/recac-test", "user.email", "agent@recac.com").Return(nil)
		mockGitClient.On("Config", "/tmp/recac-test", "user.name", "Recac Agent").Return(nil)
		mockGitClient.On("RemoteBranchExists", "/tmp/recac-test", "origin", "agent/TEST-1").Return(false, nil)
		mockGitClient.On("CheckoutNewBranch", "/tmp/recac-test", "agent/TEST-1").Return(nil)
		mockGitClient.On("Push", "/tmp/recac-test", "agent/TEST-1").Return(nil)
		_, err := SetupWorkspace(context.Background(), mockGitClient, "https://github.com/example/repo", "/tmp/recac-test", "TEST-1", "", "")
		assert.NoError(t, err)
		mockGitClient.AssertExpectations(t)
	})

	t.Run("Checks out existing stable feature branch", func(t *testing.T) {
		mockGitClient := &git.MockGitClient{}
		mockGitClient.On("RepoExists", "/tmp/recac-test").Return(true)
		mockGitClient.On("ConfigAddGlobal", "safe.directory", "/tmp/recac-test").Return(nil)
		mockGitClient.On("Config", "/tmp/recac-test", "user.email", "agent@recac.com").Return(nil)
		mockGitClient.On("Config", "/tmp/recac-test", "user.name", "Recac Agent").Return(nil)
		mockGitClient.On("RemoteBranchExists", "/tmp/recac-test", "origin", "agent/TEST-1").Return(true, nil)
		mockGitClient.On("Fetch", "/tmp/recac-test", "origin", "agent/TEST-1").Return(nil)
		mockGitClient.On("Checkout", "/tmp/recac-test", "agent/TEST-1").Return(nil)
		mockGitClient.On("Pull", "/tmp/recac-test", "origin", "agent/TEST-1").Return(nil)
		_, err := SetupWorkspace(context.Background(), mockGitClient, "https://github.com/example/repo", "/tmp/recac-test", "TEST-1", "", "")
		assert.NoError(t, err)
		mockGitClient.AssertExpectations(t)
	})
}
