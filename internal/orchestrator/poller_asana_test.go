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

func TestAsanaPoller_Poll(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/projects/test-project/tasks", r.URL.Path)
			assert.Equal(t, "name,notes,completed", r.URL.Query().Get("opt_fields"))
			assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

			response := `{
				"data": [
					{
						"gid": "task1",
						"name": "Task 1",
						"notes": "Fix the bug\nRepo: https://github.com/org/repo1",
						"completed": false
					},
					{
						"gid": "task2",
						"name": "Task 2",
						"notes": "Closed task",
						"completed": true
					},
					{
						"gid": "task3",
						"name": "Task 3",
						"notes": "No repo task",
						"completed": false
					}
				]
			}`
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(response))
		}))
		defer server.Close()

		poller := NewAsanaPoller("test-token", "test-project")
		poller.BaseURL = server.URL

		items, err := poller.Poll(ctx, logger)
		require.NoError(t, err)

		require.Len(t, items, 2)
		assert.Equal(t, "task1", items[0].ID)
		assert.Equal(t, "Task 1", items[0].Summary)
		assert.Equal(t, "https://github.com/org/repo1", items[0].RepoURL)

		assert.Equal(t, "task3", items[1].ID)
		assert.Equal(t, "Task 3", items[1].Summary)
		assert.Equal(t, "", items[1].RepoURL)
	})

	t.Run("Error_API", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`invalid token`))
		}))
		defer server.Close()

		poller := NewAsanaPoller("test-token", "test-project")
		poller.BaseURL = server.URL

		items, err := poller.Poll(ctx, logger)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "asana api error: 401")
		assert.Nil(t, items)
	})
}

func TestAsanaPoller_UpdateStatus(t *testing.T) {
	ctx := context.Background()

	t.Run("PostComment_And_Close", func(t *testing.T) {
		commentCalled := false
		closeCalled := false

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "/stories") {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "task123", strings.Split(r.URL.Path, "/")[2])

				var payload map[string]interface{}
				err := json.NewDecoder(r.Body).Decode(&payload)
				require.NoError(t, err)
				data := payload["data"].(map[string]interface{})
				assert.Equal(t, "Great work", data["text"])

				commentCalled = true
				w.WriteHeader(http.StatusCreated)
			} else if strings.Contains(r.URL.Path, "/tasks/task123") {
				assert.Equal(t, "PUT", r.Method)

				var payload map[string]interface{}
				err := json.NewDecoder(r.Body).Decode(&payload)
				require.NoError(t, err)
				data := payload["data"].(map[string]interface{})
				assert.Equal(t, true, data["completed"])

				closeCalled = true
				w.WriteHeader(http.StatusOK)
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		poller := NewAsanaPoller("test-token", "test-project")
		poller.BaseURL = server.URL

		item := WorkItem{ID: "task123"}
		err := poller.UpdateStatus(ctx, item, "Done", "Great work")

		require.NoError(t, err)
		assert.True(t, commentCalled)
		assert.True(t, closeCalled)
	})

	t.Run("Comment_Only", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "/stories") {
				w.WriteHeader(http.StatusCreated)
			} else {
				t.Fatalf("Unexpected request: %s", r.URL.Path)
			}
		}))
		defer server.Close()

		poller := NewAsanaPoller("test-token", "test-project")
		poller.BaseURL = server.URL

		item := WorkItem{ID: "task123"}
		err := poller.UpdateStatus(ctx, item, "In Progress", "Working on it")

		require.NoError(t, err)
	})
}

func TestAsanaPoller_Ping(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/projects/test-project", r.URL.Path)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		poller := NewAsanaPoller("test-token", "test-project")
		poller.BaseURL = server.URL

		err := poller.Ping(ctx)
		require.NoError(t, err)
	})

	t.Run("Fail", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer server.Close()

		poller := NewAsanaPoller("test-token", "test-project")
		poller.BaseURL = server.URL

		err := poller.Ping(ctx)
		assert.Error(t, err)
	})
}
