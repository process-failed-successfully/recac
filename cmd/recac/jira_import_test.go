package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"recac/internal/git"
	"recac/internal/jira"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJiraImportCmd(t *testing.T) {
	// 1. Setup mock JIRA server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/issue/PROJ-123" {
			w.Header().Set("Content-Type", "application/json")
			response := map[string]interface{}{
				"key": "PROJ-123",
				"fields": map[string]interface{}{
					"summary": "This is a test summary",
					"description": map[string]interface{}{
						"type":    "doc",
						"version": 1,
						"content": []map[string]interface{}{
							{
								"type": "paragraph",
								"content": []map[string]interface{}{
									{
										"type": "text",
										"text": "This is the ticket description.",
									},
								},
							},
						},
					},
				},
			}
			json.NewEncoder(w).Encode(response)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// 2. Create a temporary directory and initialize a git repo
	tmpDir, err := os.MkdirTemp("", "recac-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Change working directory to the temp dir
	originalWd, err := os.Getwd()
	require.NoError(t, err)
	err = os.Chdir(tmpDir)
	require.NoError(t, err)
	defer os.Chdir(originalWd)

	// Initialize git repo
	cmd := exec.Command("git", "init")
	err = cmd.Run()
	require.NoError(t, err)

	// 3. Configure the command
	// Redirect cobra output
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	// Set args
	rootCmd.SetArgs([]string{"jira", "import", "--id", "PROJ-123"})

	// Override jira client to use mock server
	originalJiraFactory := jiraClientFactory
	jiraClientFactory = func(ctx context.Context) (*jira.Client, error) {
		return jira.NewClient(server.URL, "testuser", "testtoken"), nil
	}
	defer func() { jiraClientFactory = originalJiraFactory }()

	// 4. Execute command
	err = rootCmd.Execute()
	assert.NoError(t, err)

	// 5. Assertions
	// Check output
	output := out.String()
	assert.Contains(t, output, "Success! Feature branch 'feature/PROJ-123-this-is-a-test-summary' created")

	// Check that the branch was created
	gitClient := git.NewClient()
	currentBranch, err := gitClient.CurrentBranch(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, "feature/PROJ-123-this-is-a-test-summary", currentBranch)

	// Check that app_spec.txt was created with the correct content
	specContent, err := os.ReadFile(filepath.Join(tmpDir, "app_spec.txt"))
	require.NoError(t, err)
	assert.Equal(t, "This is the ticket description.\n", string(specContent))
}

func TestSanitizeBranchName(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple case",
			input:    "This is a test",
			expected: "this-is-a-test",
		},
		{
			name:     "with special characters",
			input:    "This is a test!@#$%^&*()",
			expected: "this-is-a-test",
		},
		{
			name:     "with slashes",
			input:    "This/is/a/test",
			expected: "this-is-a-test",
		},
		{
			name:     "with leading/trailing spaces",
			input:    "  This is a test  ",
			expected: "this-is-a-test",
		},
		{
			name:     "long name",
			input:    "This is a very long name that should be truncated to a reasonable length",
			expected: "this-is-a-very-long-name-that-should-be-truncated",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, sanitizeBranchName(tc.input))
		})
	}
}
