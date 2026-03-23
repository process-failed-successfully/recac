package orchestrator

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
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
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		bodyStr := string(body)

		if requestCount == 0 {
			// First request: workflowStates query
			assert.Contains(t, bodyStr, "workflowStates")
			assert.Contains(t, bodyStr, "team_id")
			assert.Contains(t, bodyStr, "completed")

			response := `{
				"data": {
					"workflowStates": {
						"nodes": [
							{
								"id": "state-completed-123"
							}
						]
					}
				}
			}`
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(response))
		} else if requestCount == 1 {
			// Second request: issueUpdate mutation
			assert.Contains(t, bodyStr, "issueUpdate")
			assert.Contains(t, bodyStr, "internal-id-1")
			assert.Contains(t, bodyStr, "state-completed-123")

			response := `{
				"data": {
					"issueUpdate": {
						"success": true
					}
				}
			}`
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(response))
		} else {
			t.Fatalf("Unexpected request %d", requestCount)
		}
		requestCount++
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
	assert.Equal(t, 2, requestCount)
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

func TestLinearPoller_closeIssue_API_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal error"))
	}))
	defer server.Close()

	poller := NewLinearPoller("token", "team_id", "")
	poller.BaseURL = server.URL
	poller.Client = server.Client()

	err := poller.closeIssue(context.Background(), "issue_id")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch workflow states")
}

func TestLinearPoller_postComment_API_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal error"))
	}))
	defer server.Close()

	poller := NewLinearPoller("token", "team_id", "")
	poller.BaseURL = server.URL
	poller.Client = server.Client()

	err := poller.postComment(context.Background(), "issue_id", "comment text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to post linear comment")
}

func TestLinearPoller_postComment_GraphQL_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `{
			"errors": [
				{
					"message": "GraphQL mutation error"
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

	err := poller.postComment(context.Background(), "issue_id", "comment text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "linear comment error: GraphQL mutation error")
}

func TestLinearPoller_closeIssue_GraphQL_Error(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		bodyStr := string(body)

		if requestCount == 0 {
			assert.Contains(t, bodyStr, "workflowStates")
			response := `{
				"data": {
					"workflowStates": {
						"nodes": [
							{
								"id": "state-completed-123"
							}
						]
					}
				}
			}`
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(response))
		} else if requestCount == 1 {
			assert.Contains(t, bodyStr, "issueUpdate")
			response := `{
				"errors": [
					{
						"message": "Update failed"
					}
				]
			}`
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(response))
		}
		requestCount++
	}))
	defer server.Close()

	poller := NewLinearPoller("token", "team_id", "")
	poller.BaseURL = server.URL
	poller.Client = server.Client()

	err := poller.closeIssue(context.Background(), "issue_id")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "linear issue update error: Update failed")
}

func TestLinearPoller_closeIssue_Errors(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	poller := NewLinearPoller(

		"test-token",
		"test-team",
		"todo",
	)
	poller.BaseURL = ts.URL

	err := poller.closeIssue(context.Background(), "lin-123")
	assert.Error(t, err)

	// Test missing state ID
	ts2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":{"workflowStates":{"nodes":[]}}}`))
	}))
	defer ts2.Close()

	poller2 := NewLinearPoller(

		"test-token",
		"test-team",
		"todo",
	)
	poller2.BaseURL = ts2.URL

	err = poller2.closeIssue(context.Background(), "lin-123")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no completed workflow state found for team")

	// Test failed mutation
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var req map[string]interface{}
		json.NewDecoder(r.Body).Decode(&req)

		query, _ := req["query"].(string)
		if strings.Contains(query, "workflowStates") {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data":{"workflowStates":{"nodes":[{"id":"state-1"}]}}}`))
		} else if strings.Contains(query, "issueUpdate") {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
	})
	ts3 := httptest.NewServer(mux)
	defer ts3.Close()

	poller3 := NewLinearPoller(

		"test-token",
		"test-team",
		"todo",
	)
	poller3.BaseURL = ts3.URL

	err = poller3.closeIssue(context.Background(), "lin-123")
	assert.Error(t, err)
}

func TestLinearPoller_closeIssue_DecodeError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "10")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json"))
	}))
	defer ts.Close()

	poller := NewLinearPoller("token", "team_id", "")
	poller.BaseURL = ts.URL
	poller.Client = ts.Client()

	err := poller.closeIssue(context.Background(), "lin-123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode workflow states response")
}

func TestLinearPoller_closeIssue_UpdateDecodeError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var req map[string]interface{}
		json.NewDecoder(r.Body).Decode(&req)

		query, _ := req["query"].(string)
		if strings.Contains(query, "workflowStates") {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data":{"workflowStates":{"nodes":[{"id":"state-1"}]}}}`))
		} else if strings.Contains(query, "issueUpdate") {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("invalid json"))
		}
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	poller := NewLinearPoller("token", "team_id", "")
	poller.BaseURL = ts.URL
	poller.Client = ts.Client()

	err := poller.closeIssue(context.Background(), "lin-123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode issue update response")
}

func TestLinearPoller_closeIssue_UpdateNotSuccess(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var req map[string]interface{}
		json.NewDecoder(r.Body).Decode(&req)

		query, _ := req["query"].(string)
		if strings.Contains(query, "workflowStates") {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data":{"workflowStates":{"nodes":[{"id":"state-1"}]}}}`))
		} else if strings.Contains(query, "issueUpdate") {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data":{"issueUpdate":{"success":false}}}`))
		}
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	poller := NewLinearPoller("token", "team_id", "")
	poller.BaseURL = ts.URL
	poller.Client = ts.Client()

	err := poller.closeIssue(context.Background(), "lin-123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "linear issue update failed")
}
