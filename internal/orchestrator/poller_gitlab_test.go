package orchestrator

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGitLabPoller_Poll(t *testing.T) {
	// Mock Server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Header
		if r.Header.Get("PRIVATE-TOKEN") != "test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// List Issues
		if r.Method == "GET" && (r.URL.Path == "/api/v4/projects/owner%2Frepo/issues" || r.URL.Path == "/api/v4/projects/owner/repo/issues") {
			issues := []map[string]interface{}{
				{
					"iid":         1,
					"title":       "Test Issue 1",
					"description": "This is a test issue. Repo: https://gitlab.com/other/repo.git",
					"web_url":     "https://gitlab.com/other/repo/-/issues/1",
				},
				{
					"iid":         2,
					"title":       "Test Issue 2",
					"description": "This is another issue without explicit repo.",
					"web_url":     "https://gitlab.com/owner/repo/-/issues/2",
				},
			}
			json.NewEncoder(w).Encode(issues)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	p := NewGitLabPoller(server.URL, "test-token", "owner%2Frepo", "test-label")

	items, err := p.Poll(context.Background(), slog.New(slog.NewTextHandler(os.Stdout, nil)))
	if err != nil {
		t.Fatalf("Poll failed: %v", err)
	}
	assert.Len(t, items, 2)

	// Issue 1: Explicit Repo
	assert.Equal(t, "gl-1", items[0].ID)
	assert.Equal(t, "Test Issue 1", items[0].Summary)
	assert.Equal(t, "https://gitlab.com/other/repo", items[0].RepoURL)

	// Issue 2: Default Repo from web_url
	assert.Equal(t, "gl-2", items[1].ID)
	assert.Equal(t, "Test Issue 2", items[1].Summary)
	assert.Equal(t, "https://gitlab.com/owner/repo", items[1].RepoURL)
}

func TestGitLabPoller_UpdateStatus_Done(t *testing.T) {
	commentPosted := false
	issueClosed := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Post Comment
		if r.Method == "POST" && (r.URL.Path == "/api/v4/projects/owner%2Frepo/issues/1/notes" || r.URL.Path == "/api/v4/projects/owner/repo/issues/1/notes") {
			var payload map[string]string
			json.NewDecoder(r.Body).Decode(&payload)
			if payload["body"] == "Job Done" {
				commentPosted = true
				w.WriteHeader(http.StatusCreated)
				return
			}
		}

		// Close Issue
		if r.Method == "PUT" && (r.URL.Path == "/api/v4/projects/owner%2Frepo/issues/1" || r.URL.Path == "/api/v4/projects/owner/repo/issues/1") {
			var payload map[string]string
			json.NewDecoder(r.Body).Decode(&payload)
			if payload["state_event"] == "close" {
				issueClosed = true
				w.WriteHeader(http.StatusOK)
				return
			}
		}

		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	p := NewGitLabPoller(server.URL, "test-token", "owner%2Frepo", "test-label")

	item := WorkItem{ID: "gl-1"}
	err := p.UpdateStatus(context.Background(), item, "Done", "Job Done")

	assert.NoError(t, err)
	assert.True(t, commentPosted, "Comment should be posted")
	assert.True(t, issueClosed, "Issue should be closed")
}

func TestGitLabPoller_Ping(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v4/projects/owner%2Frepo" || r.URL.Path == "/api/v4/projects/owner/repo" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	p := NewGitLabPoller(server.URL, "test-token", "owner%2Frepo", "test-label")

	err := p.Ping(context.Background())
	assert.NoError(t, err)
}

func TestGitLabPoller_closeIssue_API_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal error"))
	}))
	defer server.Close()

	poller := NewGitLabPoller(server.URL, "token", "owner%2Frepo", "label")

	err := poller.closeIssue(context.Background(), "123")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to close issue")
}

func TestGitLabPoller_postComment_API_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal error"))
	}))
	defer server.Close()

	poller := NewGitLabPoller(server.URL, "token", "owner%2Frepo", "label")

	err := poller.postComment(context.Background(), "123", "comment")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to post comment")
}

func TestGitLabPoller_Ping_API_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal error"))
	}))
	defer server.Close()

	poller := NewGitLabPoller(server.URL, "token", "owner%2Frepo", "label")

	err := poller.Ping(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "gitlab ping failed")
}

func TestGitLabPoller_UpdateStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "notes") {
			w.WriteHeader(http.StatusOK)
		} else if r.Method == "PUT" && strings.Contains(r.URL.Path, "issues") {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer ts.Close()

	poller := NewGitLabPoller(
		ts.URL,
		"test-token",
		"test-project",
		"open",
	)

	// Test successful comment
	err := poller.UpdateStatus(context.Background(), WorkItem{ID: "gl-123"}, "Open", "test comment")
	assert.NoError(t, err)

	// Test successful close
	err = poller.UpdateStatus(context.Background(), WorkItem{ID: "gl-123"}, "Done", "")
	assert.NoError(t, err)

	// Test failed comment
	tsErr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer tsErr.Close()

	pollerErr := NewGitLabPoller(
		tsErr.URL,
		"test-token",
		"test-project",
		"open",
	)

	err = pollerErr.UpdateStatus(context.Background(), WorkItem{ID: "gl-123"}, "Open", "test comment")
	assert.Error(t, err)

	// Test failed close
	err = pollerErr.UpdateStatus(context.Background(), WorkItem{ID: "gl-123"}, "Done", "")
	assert.Error(t, err)
}

func TestGitLabPoller_Poll_RequestError(t *testing.T) {
	p := NewGitLabPoller("token", "123", "label", "http://\x00invalid")
	_, err := p.Poll(context.Background(), slog.New(slog.NewTextHandler(os.Stdout, nil)))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create request")
}

func TestGitLabPoller_Poll_ClientError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Close()
	p := NewGitLabPoller(server.URL, "token", "123", "label")
	_, err := p.Poll(context.Background(), slog.New(slog.NewTextHandler(os.Stdout, nil)))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to execute request")
}

func TestGitLabPoller_Poll_BadJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{invalid json}"))
	}))
	defer server.Close()

	p := NewGitLabPoller(server.URL, "token", "123", "label")
	_, err := p.Poll(context.Background(), slog.New(slog.NewTextHandler(os.Stdout, nil)))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode response")
}

func TestGitLabPoller_closeIssue_ClientError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Close()
	p := NewGitLabPoller(server.URL, "token", "123", "label")
	err := p.closeIssue(context.Background(), "123")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to close issue")
}

func TestGitLabPoller_postComment_ClientError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Close()
	p := NewGitLabPoller(server.URL, "token", "123", "label")
	err := p.postComment(context.Background(), "123", "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to post comment")
}

func TestGitLabPoller_Ping_ClientError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Close()
	p := NewGitLabPoller(server.URL, "token", "123", "label")
	err := p.Ping(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to reach gitlab")
}

func TestGitLabPoller_RequestErrors(t *testing.T) {
	p := NewGitLabPoller("http://\x00invalid", "token", "123", "label")
	err := p.postComment(context.Background(), "123", "test")
	assert.Error(t, err)

	err = p.closeIssue(context.Background(), "123")
	assert.Error(t, err)

	err = p.Ping(context.Background())
	assert.Error(t, err)
}


func TestGitLabPoller_UpdateStatus_CloseError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			w.WriteHeader(http.StatusCreated)
		} else if r.Method == "PUT" {
			w.WriteHeader(http.StatusBadRequest) // Fail close
		}
	}))
	defer server.Close()

	p := NewGitLabPoller(server.URL, "token", "123", "label")
	item := WorkItem{ID: "gl-1"}
	err := p.UpdateStatus(context.Background(), item, "Done", "Job Done")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to close issue: 400")
}

func TestGitLabPoller_UpdateStatus_NoComment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	p := NewGitLabPoller(server.URL, "token", "123", "label")
	item := WorkItem{ID: "gl-1"}
	err := p.UpdateStatus(context.Background(), item, "InProgress", "")
	assert.NoError(t, err)
}

func TestGitLabPoller_UpdateStatus_PostCommentError(t *testing.T) {
	p := NewGitLabPoller("http://\x00invalid", "token", "123", "label")
	item := WorkItem{ID: "gl-1"}
	err := p.UpdateStatus(context.Background(), item, "InProgress", "comment")
	assert.Error(t, err)
}
