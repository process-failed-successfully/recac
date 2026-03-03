package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLinearPoller_Poll(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "token", r.Header.Get("Authorization"))

		body, _ := io.ReadAll(r.Body)
		assert.Contains(t, string(body), "team_id")

		response := `{
			"data": {
				"issues": {
					"nodes": [
						{
							"id": "internal-id-1",
							"identifier": "ENG-1",
							"title": "Fix login bug",
							"description": "Repo: https://github.com/test/repo",
							"url": "https://linear.app/team/issue/ENG-1"
						},
						{
							"id": "internal-id-2",
							"identifier": "ENG-2",
							"title": "Add linear poller",
							"description": "Add linear support",
							"url": "https://linear.app/team/issue/ENG-2"
						}
					]
				}
			}
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	poller := NewLinearPoller("token", "team_id", "")
	poller.BaseURL = server.URL
	poller.Client = server.Client()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	items, err := poller.Poll(context.Background(), logger)

	require.NoError(t, err)
	assert.Len(t, items, 2)

	assert.Equal(t, "ENG-1", items[0].ID)
	assert.Equal(t, "Fix login bug", items[0].Summary)
	assert.Equal(t, "https://github.com/test/repo", items[0].RepoURL)
	assert.Equal(t, "internal-id-1", items[0].EnvVars["LINEAR_ISSUE_ID"])

	assert.Equal(t, "ENG-2", items[1].ID)
	assert.Equal(t, "Add linear poller", items[1].Summary)
	assert.Equal(t, "", items[1].RepoURL)
	assert.Equal(t, "internal-id-2", items[1].EnvVars["LINEAR_ISSUE_ID"])
}

func TestLinearPoller_Poll_WithLabel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		assert.Contains(t, string(body), "labels: { name: { eq: \\\"recac-agent\\\" } }")

		response := `{
			"data": {
				"issues": {
					"nodes": []
				}
			}
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	poller := NewLinearPoller("token", "team_id", "recac-agent")
	poller.BaseURL = server.URL
	poller.Client = server.Client()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	items, err := poller.Poll(context.Background(), logger)

	require.NoError(t, err)
	assert.Len(t, items, 0)
}

func TestLinearPoller_Poll_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	poller := NewLinearPoller("token", "team_id", "")
	poller.BaseURL = server.URL
	poller.Client = server.Client()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	_, err := poller.Poll(context.Background(), logger)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "linear api error")
}

func TestLinearPoller_Poll_GraphQLError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `{
			"errors": [
				{
					"message": "Invalid token"
				}
			]
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	poller := NewLinearPoller("token", "team_id", "")
	poller.BaseURL = server.URL
	poller.Client = server.Client()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	_, err := poller.Poll(context.Background(), logger)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "linear graphql error: Invalid token")
}

func TestLinearPoller_UpdateStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		assert.Contains(t, string(body), "commentCreate")
		assert.Contains(t, string(body), "internal-id-1")
		assert.Contains(t, string(body), "Test comment")

		response := `{
			"data": {
				"commentCreate": {
					"success": true
				}
			}
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	poller := NewLinearPoller("token", "team_id", "")
	poller.BaseURL = server.URL
	poller.Client = server.Client()

	item := WorkItem{
		ID: "ENG-1",
		EnvVars: map[string]string{
			"LINEAR_ISSUE_ID": "internal-id-1",
		},
	}

	err := poller.UpdateStatus(context.Background(), item, "InProgress", "Test comment")
	require.NoError(t, err)
}

func TestLinearPoller_UpdateStatus_Done(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		assert.Contains(t, string(body), "commentCreate")
		assert.Contains(t, string(body), "internal-id-1")
		assert.Contains(t, string(body), "Status changed to: Done")

		response := `{
			"data": {
				"commentCreate": {
					"success": true
				}
			}
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	poller := NewLinearPoller("token", "team_id", "")
	poller.BaseURL = server.URL
	poller.Client = server.Client()

	item := WorkItem{
		ID: "ENG-1",
		EnvVars: map[string]string{
			"LINEAR_ISSUE_ID": "internal-id-1",
		},
	}

	err := poller.UpdateStatus(context.Background(), item, "Done", "")
	require.NoError(t, err)
}

func TestLinearPoller_UpdateStatus_NoInternalID(t *testing.T) {
	poller := NewLinearPoller("token", "team_id", "")

	item := WorkItem{
		ID: "ENG-1",
	}

	err := poller.UpdateStatus(context.Background(), item, "Done", "Test comment")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "linear issue internal ID not found")
}

func TestLinearPoller_Ping(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		assert.Contains(t, string(body), "team(id: \\\"team_id\\\")")

		response := `{
			"data": {
				"team": {
					"id": "team_id"
				}
			}
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	poller := NewLinearPoller("token", "team_id", "")
	poller.BaseURL = server.URL
	poller.Client = server.Client()

	err := poller.Ping(context.Background())
	require.NoError(t, err)
}

func TestLinearPoller_Ping_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `{
			"data": {
				"team": {
					"id": ""
				}
			}
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	poller := NewLinearPoller("token", "team_id", "")
	poller.BaseURL = server.URL
	poller.Client = server.Client()

	err := poller.Ping(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "linear team 'team_id' not found")
}

func TestLinearPoller_Ping_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("Unauthorized"))
	}))
	defer server.Close()

	poller := NewLinearPoller("token", "team_id", "")
	poller.BaseURL = server.URL
	poller.Client = server.Client()

	err := poller.Ping(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "linear ping failed: 401")
}
