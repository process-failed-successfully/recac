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
	"github.com/stretchr/testify/require"
)

func TestTrelloPoller_Poll(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ctx := context.Background()

	t.Run("Success_WithListID", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/lists/list123/cards", r.URL.Path)
			assert.Equal(t, "test-key", r.URL.Query().Get("key"))
			assert.Equal(t, "test-token", r.URL.Query().Get("token"))

			cards := []map[string]interface{}{
				{
					"id":     "card1",
					"name":   "Task 1",
					"desc":   "Fix the bug\nRepo: https://github.com/org/repo1",
					"closed": false,
				},
				{
					"id":     "card2",
					"name":   "Task 2",
					"desc":   "Closed task",
					"closed": true,
				},
				{
					"id":     "card3",
					"name":   "Task 3",
					"desc":   "No repo task",
					"closed": false,
				},
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(cards)
		}))
		defer server.Close()

		poller := NewTrelloPoller("test-key", "test-token", "", "list123")
		poller.BaseURL = server.URL

		items, err := poller.Poll(ctx, logger)
		require.NoError(t, err)

		require.Len(t, items, 2)
		assert.Equal(t, "card1", items[0].ID)
		assert.Equal(t, "Task 1", items[0].Summary)
		assert.Equal(t, "https://github.com/org/repo1", items[0].RepoURL)

		assert.Equal(t, "card3", items[1].ID)
		assert.Equal(t, "Task 3", items[1].Summary)
		assert.Equal(t, "", items[1].RepoURL)
	})

	t.Run("Success_WithBoardID", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/boards/board123/cards", r.URL.Path)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`[]`))
		}))
		defer server.Close()

		poller := NewTrelloPoller("test-key", "test-token", "board123", "")
		poller.BaseURL = server.URL

		items, err := poller.Poll(ctx, logger)
		require.NoError(t, err)
		assert.Empty(t, items)
	})

	t.Run("Error_MissingIDs", func(t *testing.T) {
		poller := NewTrelloPoller("test-key", "test-token", "", "")

		items, err := poller.Poll(ctx, logger)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "either BoardID or ListID must be provided")
		assert.Nil(t, items)
	})

	t.Run("Error_API", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`invalid token`))
		}))
		defer server.Close()

		poller := NewTrelloPoller("test-key", "test-token", "board123", "")
		poller.BaseURL = server.URL

		items, err := poller.Poll(ctx, logger)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "trello api error: 401")
		assert.Nil(t, items)
	})
}

func TestTrelloPoller_UpdateStatus(t *testing.T) {
	ctx := context.Background()

	t.Run("PostComment_And_Close", func(t *testing.T) {
		commentCalled := false
		closeCalled := false

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "/actions/comments") {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "card123", strings.Split(r.URL.Path, "/")[2])
				assert.Equal(t, "Great work", r.URL.Query().Get("text"))
				commentCalled = true
				w.WriteHeader(http.StatusOK)
			} else if strings.Contains(r.URL.Path, "/cards/card123") {
				assert.Equal(t, "PUT", r.Method)
				assert.Equal(t, "true", r.URL.Query().Get("closed"))
				closeCalled = true
				w.WriteHeader(http.StatusOK)
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		poller := NewTrelloPoller("test-key", "test-token", "", "")
		poller.BaseURL = server.URL

		item := WorkItem{ID: "card123"}
		err := poller.UpdateStatus(ctx, item, "Done", "Great work")

		require.NoError(t, err)
		assert.True(t, commentCalled)
		assert.True(t, closeCalled)
	})

	t.Run("Comment_Only", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "/actions/comments") {
				w.WriteHeader(http.StatusOK)
			} else {
				t.Fatalf("Unexpected request: %s", r.URL.Path)
			}
		}))
		defer server.Close()

		poller := NewTrelloPoller("test-key", "test-token", "", "")
		poller.BaseURL = server.URL

		item := WorkItem{ID: "card123"}
		err := poller.UpdateStatus(ctx, item, "In Progress", "Working on it")

		require.NoError(t, err)
	})
}

func TestTrelloPoller_Ping(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/members/me", r.URL.Path)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		poller := NewTrelloPoller("test-key", "test-token", "", "")
		poller.BaseURL = server.URL

		err := poller.Ping(ctx)
		require.NoError(t, err)
	})

	t.Run("Fail", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer server.Close()

		poller := NewTrelloPoller("test-key", "test-token", "", "")
		poller.BaseURL = server.URL

		err := poller.Ping(ctx)
		assert.Error(t, err)
	})
}
