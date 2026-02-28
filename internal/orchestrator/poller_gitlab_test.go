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
