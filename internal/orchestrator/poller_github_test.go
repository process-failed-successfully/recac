package orchestrator

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitHubPoller_Poll(t *testing.T) {
	// Mock Server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Header
		if r.Header.Get("Authorization") != "token test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// List Issues
		if r.Method == "GET" && r.URL.Path == "/repos/owner/repo/issues" {
			issues := []map[string]interface{}{
				{
					"number": 1,
					"title":  "Test Issue 1",
					"body":   "This is a test issue. Repo: https://github.com/other/repo.git",
				},
				{
					"number": 2,
					"title":  "Test Issue 2",
					"body":   "This is another issue without explicit repo.",
				},
				{
					"number":       3,
					"title":        "PR",
					"pull_request": map[string]interface{}{},
				},
			}
			json.NewEncoder(w).Encode(issues)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	p := NewGitHubPoller("test-token", "owner", "repo", "test-label")
	p.BaseURL = server.URL // Override BaseURL

	items, err := p.Poll(context.Background(), slog.New(slog.NewTextHandler(os.Stdout, nil)))
	assert.NoError(t, err)
	assert.Len(t, items, 2)

	// Issue 1: Explicit Repo
	assert.Equal(t, "gh-1", items[0].ID)
	assert.Equal(t, "Test Issue 1", items[0].Summary)
	assert.Equal(t, "https://github.com/other/repo", items[0].RepoURL)

	// Issue 2: Default Repo
	assert.Equal(t, "gh-2", items[1].ID)
	assert.Equal(t, "Test Issue 2", items[1].Summary)
	assert.Equal(t, "https://github.com/owner/repo", items[1].RepoURL)
}

func TestGitHubPoller_UpdateStatus_Done(t *testing.T) {
	commentPosted := false
	issueClosed := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Post Comment
		if r.Method == "POST" && r.URL.Path == "/repos/owner/repo/issues/1/comments" {
			var payload map[string]string
			json.NewDecoder(r.Body).Decode(&payload)
			if payload["body"] == "Job Done" {
				commentPosted = true
				w.WriteHeader(http.StatusCreated)
				return
			}
		}

		// Close Issue
		if r.Method == "PATCH" && r.URL.Path == "/repos/owner/repo/issues/1" {
			var payload map[string]string
			json.NewDecoder(r.Body).Decode(&payload)
			if payload["state"] == "closed" {
				issueClosed = true
				w.WriteHeader(http.StatusOK)
				return
			}
		}

		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	p := NewGitHubPoller("test-token", "owner", "repo", "test-label")
	p.BaseURL = server.URL

	item := WorkItem{ID: "gh-1"}
	err := p.UpdateStatus(context.Background(), item, "Done", "Job Done")

	assert.NoError(t, err)
	assert.True(t, commentPosted, "Comment should be posted")
	assert.True(t, issueClosed, "Issue should be closed")
}

func TestGitHubPoller_UpdateStatus_InProgress(t *testing.T) {
	commentPosted := false
	issueClosed := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/repos/owner/repo/issues/1/comments" {
			commentPosted = true
			w.WriteHeader(http.StatusCreated)
			return
		}
		if r.Method == "PATCH" {
			issueClosed = true // Should not happen
		}
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	p := NewGitHubPoller("test-token", "owner", "repo", "test-label")
	p.BaseURL = server.URL

	item := WorkItem{ID: "gh-1"}
	err := p.UpdateStatus(context.Background(), item, "In Progress", "Starting")

	assert.NoError(t, err)
	assert.True(t, commentPosted, "Comment should be posted")
	assert.False(t, issueClosed, "Issue should NOT be closed")
}

func TestGitHubPoller_Ping(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/owner/repo" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	p := NewGitHubPoller("test-token", "owner", "repo", "test-label")
	p.BaseURL = server.URL

	err := p.Ping(context.Background())
	assert.NoError(t, err)
}

func TestGitHubPoller_closeIssue_API_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal error"))
	}))
	defer server.Close()

	poller := NewGitHubPoller("token", "owner", "repo", "label")
	poller.BaseURL = server.URL

	err := poller.closeIssue(context.Background(), "123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to close issue")
}

func TestGitHubPoller_postComment_API_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal error"))
	}))
	defer server.Close()

	poller := NewGitHubPoller("token", "owner", "repo", "label")
	poller.BaseURL = server.URL

	err := poller.postComment(context.Background(), "123", "comment")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to post comment")
}

func TestGitHubPoller_Ping_API_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal error"))
	}))
	defer server.Close()

	poller := NewGitHubPoller("token", "owner", "repo", "label")
	poller.BaseURL = server.URL

	err := poller.Ping(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "github ping failed")
}

func TestGitHubPoller_Poll_RequestError(t *testing.T) {
	p := NewGitHubPoller("token", "owner", "repo", "label")
	// Using a bad URL to cause http request creation/execution failure
	p.BaseURL = "http://\x00invalid"
	_, err := p.Poll(context.Background(), slog.New(slog.NewTextHandler(os.Stdout, nil)))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create request")
}

func TestGitHubPoller_Poll_ClientError(t *testing.T) {
	p := NewGitHubPoller("token", "owner", "repo", "label")
	// Use closed server to cause client.Do to fail
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Close()
	p.BaseURL = server.URL
	_, err := p.Poll(context.Background(), slog.New(slog.NewTextHandler(os.Stdout, nil)))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to execute request")
}

func TestGitHubPoller_Poll_BadJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{invalid json}"))
	}))
	defer server.Close()

	p := NewGitHubPoller("token", "owner", "repo", "label")
	p.BaseURL = server.URL
	_, err := p.Poll(context.Background(), slog.New(slog.NewTextHandler(os.Stdout, nil)))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode response")
}

func TestGitHubPoller_UpdateStatus_CloseError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			w.WriteHeader(http.StatusCreated)
		} else if r.Method == "PATCH" {
			w.WriteHeader(http.StatusBadRequest) // Fail close
		}
	}))
	defer server.Close()

	p := NewGitHubPoller("token", "owner", "repo", "label")
	p.BaseURL = server.URL
	item := WorkItem{ID: "gh-1"}
	err := p.UpdateStatus(context.Background(), item, "Done", "Job Done")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to close issue: 400")
}

func TestGitHubPoller_UpdateStatus_NoComment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	p := NewGitHubPoller("token", "owner", "repo", "label")
	p.BaseURL = server.URL
	item := WorkItem{ID: "gh-1"}
	// Empty comment should not trigger comment code
	err := p.UpdateStatus(context.Background(), item, "InProgress", "")
	assert.NoError(t, err)
}

func TestGitHubPoller_closeIssue_ClientError(t *testing.T) {
	p := NewGitHubPoller("token", "owner", "repo", "label")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Close()
	p.BaseURL = server.URL
	err := p.closeIssue(context.Background(), "123")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to close issue")
}

func TestGitHubPoller_postComment_ClientError(t *testing.T) {
	p := NewGitHubPoller("token", "owner", "repo", "label")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Close()
	p.BaseURL = server.URL
	err := p.postComment(context.Background(), "123", "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to post comment")
}

func TestGitHubPoller_Ping_ClientError(t *testing.T) {
	p := NewGitHubPoller("token", "owner", "repo", "label")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Close()
	p.BaseURL = server.URL
	err := p.Ping(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to reach github")
}

func TestGitHubPoller_RequestErrors(t *testing.T) {
	p := NewGitHubPoller("token", "owner", "repo", "label")
	p.BaseURL = "http://\x00invalid"
	err := p.postComment(context.Background(), "123", "test")
	assert.Error(t, err)

	err = p.closeIssue(context.Background(), "123")
	assert.Error(t, err)

	err = p.Ping(context.Background())
	assert.Error(t, err)
}


func TestGitHubPoller_UpdateStatus_PostCommentError(t *testing.T) {
	p := NewGitHubPoller("token", "owner", "repo", "label")
	p.BaseURL = "http://\x00invalid"
	item := WorkItem{ID: "gh-1"}
	err := p.UpdateStatus(context.Background(), item, "InProgress", "comment")
	assert.Error(t, err)
}
