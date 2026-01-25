package orchestrator

import (
	"context"
	"errors"
	"recac/internal/jira"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock Jira Client
type MockJiraClient struct {
	mock.Mock
}

func (m *MockJiraClient) SearchIssues(ctx context.Context, jql string) ([]map[string]interface{}, error) {
	args := m.Called(ctx, jql)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]map[string]interface{}), args.Error(1)
}

func (m *MockJiraClient) GetBlockers(issue map[string]interface{}) []string {
	args := m.Called(issue)
	return args.Get(0).([]string)
}

func (m *MockJiraClient) ParseDescription(issue map[string]interface{}) string {
	args := m.Called(issue)
	return args.String(0)
}

func (m *MockJiraClient) AddComment(ctx context.Context, issueID string, comment string) error {
	args := m.Called(ctx, issueID, comment)
	return args.Error(0)
}

func (m *MockJiraClient) SmartTransition(ctx context.Context, issueID string, status string) error {
	args := m.Called(ctx, issueID, status)
	return args.Error(0)
}

func TestExtractRepoURL(t *testing.T) {
	// This regex is a simplified version for testing purposes.
	// The real regex is in the jira package.
	testRegex := regexp.MustCompile(`(?i)Repo: (https?://\S+)`)

	testCases := []struct {
		name     string
		text     string
		expected string
	}{
		{"Valid HTTPS URL", "some text\nRepo: https://github.com/user/repo\nmore text", "https://github.com/user/repo"},
		{"Valid HTTPS URL with .git", "some text\nRepo: https://github.com/user/repo.git\nmore text", "https://github.com/user/repo"},
		{"Standard HTTPS", "Please fix this. Repo: https://github.com/org/repo.git", "https://github.com/org/repo"},
		{"Standard HTTP", "Repo: http://github.com/org/repo", "http://github.com/org/repo"},
		{"Case Insensitive", "repo: https://github.com/org/repo", "https://github.com/org/repo"},
		{"In middle of text", "The repo is Repo: https://github.com/foo/bar and it is cool.", "https://github.com/foo/bar"},
		{"No URL", "just some text without any url", ""},
		{"Empty String", "", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, extractRepoURL(tc.text, testRegex))
		})
	}
}

func TestJiraPoller_UpdateStatus(t *testing.T) {
	ctx := context.Background()
	item := WorkItem{ID: "PROJ-123"}

	t.Run("Comment and Status", func(t *testing.T) {
		mockClient := new(MockJiraClient)
		poller := NewJiraPoller(mockClient, "")

		mockClient.On("AddComment", ctx, "PROJ-123", "an update").Return(nil)
		mockClient.On("SmartTransition", ctx, "PROJ-123", "Done").Return(nil)

		err := poller.UpdateStatus(ctx, item, "Done", "an update")
		assert.NoError(t, err)
		mockClient.AssertExpectations(t)
	})

	t.Run("Status Only", func(t *testing.T) {
		mockClient := new(MockJiraClient)
		poller := NewJiraPoller(mockClient, "")

		mockClient.On("SmartTransition", ctx, "PROJ-123", "In Progress").Return(nil)

		err := poller.UpdateStatus(ctx, item, "In Progress", "")
		assert.NoError(t, err)
		mockClient.AssertNotCalled(t, "AddComment", mock.Anything, mock.Anything, mock.Anything)
		mockClient.AssertExpectations(t)
	})

	t.Run("Transition Fails", func(t *testing.T) {
		mockClient := new(MockJiraClient)
		poller := NewJiraPoller(mockClient, "")
		expectedErr := errors.New("transition failed")

		mockClient.On("SmartTransition", ctx, "PROJ-123", "Failed").Return(expectedErr)

		err := poller.UpdateStatus(ctx, item, "Failed", "")
		assert.Equal(t, expectedErr, err)
		mockClient.AssertExpectations(t)
	})
}

// Helper to create a mock Jira issue
func mockIssue(key, summary, description string) map[string]interface{} {
	return map[string]interface{}{
		"key": key,
		"fields": map[string]interface{}{
			"summary": summary,
		},
		// The real description is parsed from fields, but our mock doesn't need that detail
	}
}

func TestJiraPoller_Poll(t *testing.T) {
	ctx := context.Background()
	originalRegex := jira.RepoRegex
	defer func() { jira.RepoRegex = originalRegex }()
	jira.RepoRegex = regexp.MustCompile(`(?i)Repo: (https?://\S+)`)

	// Mocks for different issues
	issue1 := mockIssue("PROJ-1", "Task 1", "Repo: https://github.com/test/repo1")
	issue2 := mockIssue("PROJ-2", "Task 2", "No repo here")
	issue3 := mockIssue("PROJ-3", "Task 3", "Repo: https://github.com/test/repo3")
	issue4 := mockIssue("PROJ-4", "Task 4", "Repo: https://github.com/test/repo4")

	t.Run("Success - Finds Actionable Items", func(t *testing.T) {
		mockClient := new(MockJiraClient)
		poller := NewJiraPoller(mockClient, "status = 'To Do'")

		issues := []map[string]interface{}{issue1, issue2, issue3, issue4}
		mockClient.On("SearchIssues", ctx, "status = 'To Do'").Return(issues, nil)

		// PROJ-1: Ready to go
		mockClient.On("GetBlockers", issue1).Return([]string{})
		mockClient.On("ParseDescription", issue1).Return("Repo: https://github.com/test/repo1")

		// PROJ-2: No repo
		mockClient.On("GetBlockers", issue2).Return([]string{})
		mockClient.On("ParseDescription", issue2).Return("No repo here")

		// PROJ-3: Blocked by PROJ-4 (internal)
		mockClient.On("GetBlockers", issue3).Return([]string{"PROJ-4 (To Do)"})

		// PROJ-4: Blocked externally
		mockClient.On("GetBlockers", issue4).Return([]string{"EXT-1 (In Progress)"})

		workItems, err := poller.Poll(ctx, nil)

		assert.NoError(t, err)
		assert.Len(t, workItems, 1)
		assert.Equal(t, "PROJ-1", workItems[0].ID)
		assert.Equal(t, "https://github.com/test/repo1", workItems[0].RepoURL)
		assert.Equal(t, "PROJ-1", workItems[0].EnvVars["JIRA_TICKET"])
		mockClient.AssertExpectations(t)
	})

	t.Run("Search Fails", func(t *testing.T) {
		mockClient := new(MockJiraClient)
		poller := NewJiraPoller(mockClient, "status = 'To Do'")
		expectedErr := errors.New("jira is down")

		mockClient.On("SearchIssues", ctx, "status = 'To Do'").Return(nil, expectedErr)

		workItems, err := poller.Poll(ctx, nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "jira is down")
		assert.Nil(t, workItems)
		mockClient.AssertExpectations(t)
	})

	t.Run("No Issues Found", func(t *testing.T) {
		mockClient := new(MockJiraClient)
		poller := NewJiraPoller(mockClient, "status = 'To Do'")

		mockClient.On("SearchIssues", ctx, "status = 'To Do'").Return([]map[string]interface{}{}, nil)

		workItems, err := poller.Poll(ctx, nil)

		assert.NoError(t, err)
		assert.Empty(t, workItems)
		mockClient.AssertExpectations(t)
	})

	t.Run("Default JQL Used", func(t *testing.T) {
		mockClient := new(MockJiraClient)
		poller := NewJiraPoller(mockClient, "") // Empty JQL

		mockClient.On("SearchIssues", ctx, "statusCategory != Done ORDER BY created ASC").Return([]map[string]interface{}{}, nil)

		_, err := poller.Poll(ctx, nil)
		assert.NoError(t, err)
		assert.Equal(t, "statusCategory != Done ORDER BY created ASC", poller.JQL)
		mockClient.AssertExpectations(t)
	})

	t.Run("Extract Required Features", func(t *testing.T) {
		mockClient := new(MockJiraClient)
		poller := NewJiraPoller(mockClient, "status = 'To Do'")

		desc := "Repo: https://github.com/test/repo\nREQUIRED FEATURES:\n- Feature A\n* Feature B"
		issue := mockIssue("PROJ-FEAT", "Feature Request", desc)

		mockClient.On("SearchIssues", ctx, "status = 'To Do'").Return([]map[string]interface{}{issue}, nil)
		mockClient.On("GetBlockers", issue).Return([]string{})
		mockClient.On("ParseDescription", issue).Return(desc)

		workItems, err := poller.Poll(ctx, nil)

		assert.NoError(t, err)
		assert.Len(t, workItems, 1)
		assert.Contains(t, workItems[0].EnvVars, "RECAC_INJECTED_FEATURES")
		assert.Contains(t, workItems[0].EnvVars["RECAC_INJECTED_FEATURES"], "Feature A")
		assert.Contains(t, workItems[0].EnvVars["RECAC_INJECTED_FEATURES"], "Feature B")
		mockClient.AssertExpectations(t)
	})
}