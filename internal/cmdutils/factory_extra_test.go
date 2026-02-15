package cmdutils

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetupWorkspace_GithubTokenFallback(t *testing.T) {
	// Case 1: GITHUB_API_KEY is set (Priority)
	t.Run("GITHUB_API_KEY takes precedence", func(t *testing.T) {
		os.Setenv("GITHUB_API_KEY", "api-key")
		os.Setenv("GITHUB_TOKEN", "token-key")
		defer os.Unsetenv("GITHUB_API_KEY")
		defer os.Unsetenv("GITHUB_TOKEN")

		var clonedURL string
		client := &MockGitClient{
			cloneFn: func(ctx context.Context, repoURL, directory string) error {
				clonedURL = repoURL
				return nil
			},
		}

		_, err := SetupWorkspace(context.Background(), client, "https://github.com/example/repo", "/tmp", "ID", "", "")
		assert.NoError(t, err)
		// Expectation: It should use api-key
		assert.Equal(t, "https://api-key@github.com/example/repo", clonedURL)
	})

	// Case 2: GITHUB_TOKEN is set, API_KEY missing (Fallback)
	t.Run("GITHUB_TOKEN fallback", func(t *testing.T) {
		os.Unsetenv("GITHUB_API_KEY")
		os.Setenv("GITHUB_TOKEN", "token-key")
		defer os.Unsetenv("GITHUB_TOKEN")

		var clonedURL string
		client := &MockGitClient{
			cloneFn: func(ctx context.Context, repoURL, directory string) error {
				clonedURL = repoURL
				return nil
			},
		}

		_, err := SetupWorkspace(context.Background(), client, "https://github.com/example/repo", "/tmp", "ID", "", "")
		assert.NoError(t, err)

		assert.Equal(t, "https://token-key@github.com/example/repo", clonedURL)
	})
}
