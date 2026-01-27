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
					"number": 3,
					"title":  "PR",
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
