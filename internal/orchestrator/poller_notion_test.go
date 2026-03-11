package orchestrator

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotionPoller_Poll(t *testing.T) {
	// Mock Server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/databases/test-db/query", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		assert.Equal(t, "2022-06-28", r.Header.Get("Notion-Version"))

		// Decode the payload
		var payload map[string]interface{}
		json.NewDecoder(r.Body).Decode(&payload)

		// Create mock response
		resp := map[string]interface{}{
			"results": []map[string]interface{}{
				{
					"id": "page-123",
					"properties": map[string]interface{}{
						"Name": map[string]interface{}{
							"title": []map[string]interface{}{
								{
									"plain_text": "Implement login",
								},
							},
						},
						"Description": map[string]interface{}{
							"rich_text": []map[string]interface{}{
								{
									"plain_text": "Add OAuth2 support.\nRepo: https://github.com/org/repo.git",
								},
							},
						},
						"RepoURL": map[string]interface{}{
							"rich_text": []map[string]interface{}{
								{
									"plain_text": "https://github.com/org/repo.git",
								},
							},
						},
					},
				},
				{
					"id": "page-456",
					"properties": map[string]interface{}{
						"Title": map[string]interface{}{
							"title": []map[string]interface{}{
								{
									"plain_text": "Fix bug",
								},
							},
						},
					},
				},
			},
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	poller := NewNotionPoller("test-token", "test-db", "recac-agent")
	poller.BaseURL = ts.URL // Override for testing

	ctx := context.Background()
	items, err := poller.Poll(ctx, nil)

	assert.NoError(t, err)
	assert.Len(t, items, 2)

	assert.Equal(t, "page-123", items[0].ID)
	assert.Equal(t, "Implement login", items[0].Summary)
	assert.Equal(t, "Add OAuth2 support.\nRepo: https://github.com/org/repo.git", items[0].Description)
	assert.Equal(t, "https://github.com/org/repo.git", items[0].RepoURL)
	assert.Equal(t, "page-123", items[0].EnvVars["NOTION_PAGE_ID"])

	assert.Equal(t, "page-456", items[1].ID)
	assert.Equal(t, "Fix bug", items[1].Summary)
	assert.Equal(t, "", items[1].RepoURL)
}

func TestNotionPoller_Ping(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/databases/test-db", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	poller := NewNotionPoller("test-token", "test-db", "recac-agent")
	poller.BaseURL = ts.URL

	err := poller.Ping(context.Background())
	assert.NoError(t, err)
}

func TestNotionPoller_Ping_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	poller := NewNotionPoller("invalid-token", "test-db", "recac-agent")
	poller.BaseURL = ts.URL

	err := poller.Ping(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "notion ping failed with status: 401")
}

func TestNotionPoller_UpdateStatus_Done(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/pages/page-123", r.URL.Path)
		assert.Equal(t, "PATCH", r.Method)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	poller := NewNotionPoller("test-token", "test-db", "recac-agent")
	poller.BaseURL = ts.URL

	item := WorkItem{ID: "page-123"}
	err := poller.UpdateStatus(context.Background(), item, "Done", "")
	assert.NoError(t, err)
}

func TestNotionPoller_UpdateStatus_Comment(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/pages/page-123" {
			w.WriteHeader(http.StatusOK)
		} else if r.URL.Path == "/comments" {
			assert.Equal(t, "POST", r.Method)
			var payload map[string]interface{}
			json.NewDecoder(r.Body).Decode(&payload)

			parent := payload["parent"].(map[string]interface{})
			assert.Equal(t, "page-123", parent["page_id"])

			w.WriteHeader(http.StatusOK)
		}
	}))
	defer ts.Close()

	poller := NewNotionPoller("test-token", "test-db", "recac-agent")
	poller.BaseURL = ts.URL

	item := WorkItem{ID: "page-123"}
	err := poller.UpdateStatus(context.Background(), item, "In Progress", "Started working on this.")
	assert.NoError(t, err)
}

func TestNotionPoller_updatePageStatus_API_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal error"))
	}))
	defer server.Close()

	poller := NewNotionPoller("token", "db_id", "")
	poller.BaseURL = server.URL

	err := poller.updatePageStatus(context.Background(), "page_id", "Done")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update notion page")
}

func TestNotionPoller_addComment_API_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal error"))
	}))
	defer server.Close()

	poller := NewNotionPoller("token", "db_id", "")
	poller.BaseURL = server.URL

	err := poller.addComment(context.Background(), "page_id", "comment text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to post notion comment")
}

func TestNotionPoller_UpdateStatus_UpdateFailure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	poller := NewNotionPoller(
		ts.URL,
		"test-token",
		"test-db-id",
	)

	err := poller.UpdateStatus(context.Background(), WorkItem{ID: "test-id"}, "Done", "")
	assert.Error(t, err)
}

func TestNotionPoller_UpdateStatus_CommentFailure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "pages") {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer ts.Close()

	poller := NewNotionPoller(
		ts.URL,
		"test-token",
		"test-db-id",
	)

	err := poller.UpdateStatus(context.Background(), WorkItem{ID: "test-id"}, "Done", "test comment")
	assert.Error(t, err)
}
