package orchestrator

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockJiraClient provides a mock implementation of the jira.Client for testing.
type mockJiraClient struct {
	issues          []map[string]interface{}
	searchErr       error
	transitionErr   error
	commentErr      error
	blockers        map[string][]string
	descriptions    map[string]string
	transitions     map[string][]map[string]interface{}
	getTransitionsErr error
}

func (m *mockJiraClient) SearchIssues(ctx context.Context, jql string) ([]map[string]interface{}, error) {
	if m.searchErr != nil {
		return nil, m.searchErr
	}
	return m.issues, nil
}

func (m *mockJiraClient) SmartTransition(ctx context.Context, ticketID, targetNameOrID string) error {
	return m.transitionErr
}

func (m *mockJiraClient) AddComment(ctx context.Context, ticketID, commentText string) error {
	return m.commentErr
}

func (m *mockJiraClient) GetBlockers(ticket map[string]interface{}) []string {
	key, _ := ticket["key"].(string)
	return m.blockers[key]
}

func (m *mockJiraClient) ParseDescription(data map[string]interface{}) string {
	key, _ := data["key"].(string)
	return m.descriptions[key]
}

func (m *mockJiraClient) GetTransitions(ctx context.Context, ticketID string) ([]map[string]interface{}, error) {
	if m.getTransitionsErr != nil {
		return nil, m.getTransitionsErr
	}
	return m.transitions[ticketID], nil
}

func TestJiraPoller_Poll_Success(t *testing.T) {
	mockClient := &mockJiraClient{
		issues: []map[string]interface{}{
			{
				"key": "TEST-1",
				"fields": map[string]interface{}{
					"summary": "Task 1",
				},
			},
			{
				"key": "TEST-2",
				"fields": map[string]interface{}{
					"summary": "Task 2",
				},
			},
		},
		descriptions: map[string]string{
			"TEST-1": "Repo: https://github.com/test/repo1",
			"TEST-2": "Repo: https://github.com/test/repo2",
		},
	}

	poller := NewJiraPoller(mockClient, "status = 'To Do'")
	items, err := poller.Poll(context.Background())

	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, "TEST-1", items[0].ID)
	assert.Equal(t, "https://github.com/test/repo1", items[0].RepoURL)
	assert.Equal(t, "TEST-2", items[1].ID)
	assert.Equal(t, "https://github.com/test/repo2", items[1].RepoURL)
}

func TestJiraPoller_Poll_Scenarios(t *testing.T) {
	testCases := []struct {
		name          string
		setupClient   func() *mockJiraClient
		jql           string
		expectedItems int
		expectedErr   string
	}{
		{
			name: "Search Error",
			setupClient: func() *mockJiraClient {
				return &mockJiraClient{searchErr: errors.New("jira down")}
			},
			expectedItems: 0,
			expectedErr:   "failed to search issues: jira down",
		},
		{
			name: "No Issues Found",
			setupClient: func() *mockJiraClient {
				return &mockJiraClient{issues: []map[string]interface{}{}}
			},
			expectedItems: 0,
			expectedErr:   "",
		},
		{
			name: "Issue without Repo URL",
			setupClient: func() *mockJiraClient {
				return &mockJiraClient{
					issues: []map[string]interface{}{
						{"key": "TEST-1"},
					},
					descriptions: map[string]string{
						"TEST-1": "No repo here",
					},
				}
			},
			expectedItems: 0,
		},
		{
			name: "Issue with Blocker",
			setupClient: func() *mockJiraClient {
				return &mockJiraClient{
					issues: []map[string]interface{}{
						{"key": "TEST-1"},
					},
					descriptions: map[string]string{
						"TEST-1": "Repo: https://github.com/test/repo1",
					},
					blockers: map[string][]string{
						"TEST-1": {"DEV-123 (In Progress)"},
					},
				}
			},
			expectedItems: 0,
		},
		{
			name: "Default JQL",
			setupClient: func() *mockJiraClient {
				return &mockJiraClient{
					issues: []map[string]interface{}{
						{
							"key": "TEST-1",
							"fields": map[string]interface{}{
								"summary": "Task 1",
							},
						},
					},
					descriptions: map[string]string{
						"TEST-1": "Repo: https://github.com/test/repo1",
					},
				}
			},
			jql:           "", // Empty JQL should trigger default
			expectedItems: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockClient := tc.setupClient()
			poller := NewJiraPoller(mockClient, tc.jql)
			items, err := poller.Poll(context.Background())

			if tc.expectedErr != "" {
				require.EqualError(t, err, tc.expectedErr)
			} else {
				require.NoError(t, err)
				require.Len(t, items, tc.expectedItems)
			}
		})
	}
}


func TestExtractRepoURL(t *testing.T) {
	testCases := []struct {
		name     string
		text     string
		expected string
	}{
		{"Valid URL", "Repo: https://github.com/test/repo.git", "https://github.com/test/repo"},
		{"No Repo", "Some text without a repo", ""},
		{"Case Insensitive", "repo: http://gitlab.com/another/repo", "http://gitlab.com/another/repo"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, extractRepoURL(tc.text))
		})
	}
}

func TestJiraPoller_Claim(t *testing.T) {
	mockClient := &mockJiraClient{}
	poller := NewJiraPoller(mockClient, "")
	item := WorkItem{ID: "TEST-1"}
	err := poller.Claim(context.Background(), item)
	require.NoError(t, err)

	// Test error case
	mockClient.transitionErr = errors.New("transition failed")
	err = poller.Claim(context.Background(), item)
	require.Error(t, err)
}

func TestJiraPoller_UpdateStatus(t *testing.T) {
	mockClient := &mockJiraClient{}
	poller := NewJiraPoller(mockClient, "")
	item := WorkItem{ID: "TEST-1"}

	// Test comment and transition
	err := poller.UpdateStatus(context.Background(), item, "Done", "Work complete")
	require.NoError(t, err)

	// Test only comment
	err = poller.UpdateStatus(context.Background(), item, "", "Another comment")
	require.NoError(t, err)

	// Test error case
	mockClient.commentErr = errors.New("comment failed")
	err = poller.UpdateStatus(context.Background(), item, "", "Bad comment")
	require.NoError(t, err) // AddComment error is ignored

	mockClient.transitionErr = errors.New("transition failed")
	err = poller.UpdateStatus(context.Background(), item, "Failed", "")
	require.Error(t, err)
}
