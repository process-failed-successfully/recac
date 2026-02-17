package orchestrator

import (
	"context"
	"recac/internal/jira"
	"testing"
	"github.com/stretchr/testify/mock"
)

func TestExtractRequiredFeatures_Concurrent(t *testing.T) {
	// Simulate concurrent usage of extractRequiredFeatures
	// which now uses package-level regexes.

	concurrency := 100
	done := make(chan bool)

	// Sample text that triggers regex matches
	text := `
Some description here.
REQUIRED FEATURES:
- Feature 1: Login
- Feature 2: Logout
* Feature 3: Register
Some more text.
`

	for i := 0; i < concurrency; i++ {
		go func() {
			features := extractRequiredFeatures(text)
			// Basic assertions to ensure it ran correctly
			if len(features) != 3 {
				t.Errorf("Expected 3 features, got %d", len(features))
			}
			done <- true
		}()
	}

	for i := 0; i < concurrency; i++ {
		<-done
	}
}

func TestExtractRepoURL_Concurrent(t *testing.T) {
	concurrency := 100
	done := make(chan bool)

	text := "Repo: https://github.com/test/repo"
	repoRegex := jira.RepoRegex // Use package level regex if available, or pass nil to function if it handles it.
	// In poller_jira.go: extractRepoURL(text string, repoRegex *regexp.Regexp) string

	for i := 0; i < concurrency; i++ {
		go func() {
			url := extractRepoURL(text, repoRegex)
			if url != "https://github.com/test/repo" {
				t.Errorf("Expected url, got %s", url)
			}
			done <- true
		}()
	}

	for i := 0; i < concurrency; i++ {
		<-done
	}
}

func TestJiraPoller_Poll_Concurrent(t *testing.T) {
	// This test checks for race conditions in Poll method
	concurrency := 10
	done := make(chan bool)

	mockClient := new(MockJiraClient)
	// We need to set up expectation for SearchIssues
	// We use mock.Anything for arguments because concurrent calls might result in different order or exact timing
	mockClient.On("SearchIssues", mock.Anything, mock.Anything).Return([]map[string]interface{}{}, nil)

	poller := NewJiraPoller(mockClient, "") // Empty JQL triggers the potential race condition

	for i := 0; i < concurrency; i++ {
		go func() {
			_, _ = poller.Poll(context.Background(), nil)
			done <- true
		}()
	}

	for i := 0; i < concurrency; i++ {
		<-done
	}
}
