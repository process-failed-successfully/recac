package orchestrator

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

// --- Mocks ---

type mockJiraClient struct {
	issues         []map[string]interface{}
	searchErr      error
	transitionErr  error
	addCommentErr  error
	smartStatus    map[string]string // Track smart transitions
	blockers       map[string][]string
	descriptions   map[string]string
	comments       map[string]string
}

func (m *mockJiraClient) SearchIssues(ctx context.Context, jql string) ([]map[string]interface{}, error) {
	return m.issues, m.searchErr
}

func (m *mockJiraClient) SmartTransition(ctx context.Context, issueID string, status string) error {
	if m.smartStatus == nil {
		m.smartStatus = make(map[string]string)
	}
	m.smartStatus[issueID] = status
	return m.transitionErr
}

func (m *mockJiraClient) AddComment(ctx context.Context, issueID, comment string) error {
	if m.comments == nil {
		m.comments = make(map[string]string)
	}
	m.comments[issueID] = comment
	return m.addCommentErr
}

func (m *mockJiraClient) GetBlockers(issue map[string]interface{}) []string {
	key, _ := issue["key"].(string)
	return m.blockers[key]
}

func (m *mockJiraClient) ParseDescription(issue map[string]interface{}) string {
	key, _ := issue["key"].(string)
	return m.descriptions[key]
}

// --- Tests ---

func TestNewJiraPoller(t *testing.T) {
	client := &mockJiraClient{}
	poller := NewJiraPoller(client, "test jql")
	if poller == nil {
		t.Fatal("NewJiraPoller returned nil")
	}
	if poller.Client != client {
		t.Error("client not set correctly")
	}
	if poller.JQL != "test jql" {
		t.Errorf("expected jql 'test jql', got '%s'", poller.JQL)
	}
}

func TestJiraPoller_Poll(t *testing.T) {
	ctx := context.Background()

	t.Run("search error", func(t *testing.T) {
		client := &mockJiraClient{searchErr: errors.New("search failed")}
		poller := NewJiraPoller(client, "test")
		_, err := poller.Poll(ctx)
		if err == nil {
			t.Fatal("expected an error but got none")
		}
	})

	t.Run("no issues", func(t *testing.T) {
		client := &mockJiraClient{issues: []map[string]interface{}{}}
		poller := NewJiraPoller(client, "test")
		items, err := poller.Poll(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(items) != 0 {
			t.Errorf("expected 0 items, got %d", len(items))
		}
	})

	t.Run("no repo url", func(t *testing.T) {
		client := &mockJiraClient{
			issues: []map[string]interface{}{
				{"key": "TEST-1", "fields": map[string]interface{}{"summary": "s1"}},
			},
			descriptions: map[string]string{"TEST-1": "no repo"},
		}
		poller := NewJiraPoller(client, "")
		items, err := poller.Poll(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(items) != 0 {
			t.Errorf("expected 0 items for issue with no repo, got %d", len(items))
		}
	})

	t.Run("with external blockers", func(t *testing.T) {
		client := &mockJiraClient{
			issues: []map[string]interface{}{
				{"key": "TEST-1", "fields": map[string]interface{}{"summary": "s1"}},
			},
			descriptions: map[string]string{"TEST-1": "Repo: http://a.b"},
			blockers:     map[string][]string{"TEST-1": {"EXT-1 (In Progress)"}},
		}
		poller := NewJiraPoller(client, "")
		items, err := poller.Poll(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(items) != 0 {
			t.Errorf("expected 0 items for blocked issue, got %d", len(items))
		}
	})

	t.Run("successful poll with internal dependency", func(t *testing.T) {
		client := &mockJiraClient{
			issues: []map[string]interface{}{
				{"key": "TEST-1", "fields": map[string]interface{}{"summary": "Root Task"}},
				{"key": "TEST-2", "fields": map[string]interface{}{"summary": "Blocked Task"}},
			},
			descriptions: map[string]string{
				"TEST-1": "Repo: http://a.b",
				"TEST-2": "Repo: http://a.b",
			},
			blockers: map[string][]string{"TEST-2": {"TEST-1 (To Do)"}},
		}
		poller := NewJiraPoller(client, "")
		items, err := poller.Poll(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(items) != 1 {
			t.Fatalf("expected 1 ready item, got %d", len(items))
		}
		if items[0].ID != "TEST-1" {
			t.Errorf("expected ready item to be TEST-1, got %s", items[0].ID)
		}
	})
}

func TestJiraPoller_Claim(t *testing.T) {
	client := &mockJiraClient{}
	poller := NewJiraPoller(client, "")
	err := poller.Claim(context.Background(), WorkItem{ID: "TEST-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status := client.smartStatus["TEST-1"]; status != "In Progress" {
		t.Errorf("expected status 'In Progress', got '%s'", status)
	}
}

func TestJiraPoller_UpdateStatus(t *testing.T) {
	t.Run("with comment", func(t *testing.T) {
		client := &mockJiraClient{}
		poller := NewJiraPoller(client, "")
		err := poller.UpdateStatus(context.Background(), WorkItem{ID: "TEST-1"}, "Done", "all good")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if client.comments["TEST-1"] != "all good" {
			t.Error("comment was not added")
		}
		if client.smartStatus["TEST-1"] != "Done" {
			t.Error("status was not transitioned")
		}
	})
	t.Run("no comment", func(t *testing.T) {
		client := &mockJiraClient{}
		poller := NewJiraPoller(client, "")
		_ = poller.UpdateStatus(context.Background(), WorkItem{ID: "TEST-1"}, "Done", "")
		if _, exists := client.comments["TEST-1"]; exists {
			t.Error("comment was added but should not have been")
		}
	})
	t.Run("no status", func(t *testing.T) {
		client := &mockJiraClient{}
		poller := NewJiraPoller(client, "")
		_ = poller.UpdateStatus(context.Background(), WorkItem{ID: "TEST-1"}, "", "comment")
		if _, exists := client.smartStatus["TEST-1"]; exists {
			t.Error("status was transitioned but should not have been")
		}
	})
}

func Test_extractRepoURL(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{"http", "Repo: http://github.com/user/repo", "http://github.com/user/repo"},
		{"https", "Repo: https://github.com/user/repo.git", "https://github.com/user/repo"},
		{"case insensitive", "repo: https://gitlab.com/a/b", "https://gitlab.com/a/b"},
		{"no repo", "some text", ""},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractRepoURL(tt.text); got != tt.want {
				t.Errorf("extractRepoURL() = %v, want %v", got, tt.want)
			}
		})
	}
}
func TestJiraPoller_Poll_DefaultJQL(t *testing.T) {
    client := &mockJiraClient{}
    poller := NewJiraPoller(client, "") // Empty JQL
    _, _ = poller.Poll(context.Background())
    expectedJQL := "statusCategory != Done ORDER BY created ASC"
    if poller.JQL != expectedJQL {
        t.Errorf("expected default JQL to be '%s', got '%s'", expectedJQL, poller.JQL)
    }
}

func TestJiraPoller_GetBlockers(t *testing.T) {
    client := &mockJiraClient{
        blockers: map[string][]string{
            "TEST-1": {"DEP-1 (In Progress)"},
        },
    }
    issue := map[string]interface{}{"key": "TEST-1"}
    blockers := client.GetBlockers(issue)
    if !reflect.DeepEqual(blockers, []string{"DEP-1 (In Progress)"}) {
        t.Errorf("GetBlockers returned unexpected result: %v", blockers)
    }
}
