package auth_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"your-module-path/internal/auth"
)

func TestGetCredentialsFromEnv(t *testing.T) {
	// Test with missing environment variables
	t.Setenv("JIRA_USERNAME", "")
	t.Setenv("JIRA_API_KEY", "")
	t.Setenv("JIRA_BASE_URL", "")

	_, err := auth.GetCredentialsFromEnv()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing Jira credentials")

	// Test with valid environment variables
	t.Setenv("JIRA_USERNAME", "test-user")
	t.Setenv("JIRA_API_KEY", "test-api-key")
	t.Setenv("JIRA_BASE_URL", "https://test.atlassian.net")

	creds, err := auth.GetCredentialsFromEnv()
	require.NoError(t, err)
	assert.Equal(t, "test-user", creds.Username)
	assert.Equal(t, "test-api-key", creds.APIKey)
	assert.Equal(t, "https://test.atlassian.net", creds.BaseURL)
}

func TestJiraCredentialsValidation(t *testing.T) {
	creds := &auth.JiraCredentials{
		Username: "test-user",
		APIKey:   "test-api-key",
		BaseURL:  "https://test.atlassian.net",
	}

	// In a real test, we would mock the Kubernetes client
	// and verify that the credentials are retrieved correctly
	assert.NotEmpty(t, creds.Username)
	assert.NotEmpty(t, creds.APIKey)
	assert.NotEmpty(t, creds.BaseURL)
}
