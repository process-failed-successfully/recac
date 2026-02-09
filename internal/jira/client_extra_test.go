package jira

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClient_GetBlockerKeys(t *testing.T) {
	client := NewClient("http://jira.local", "user", "token")

	// Case 1: Blocked by In Progress ticket
	ticket := map[string]interface{}{
		"fields": map[string]interface{}{
			"issuelinks": []interface{}{
				map[string]interface{}{
					"type": map[string]interface{}{"inward": "is blocked by"},
					"inwardIssue": map[string]interface{}{
						"key": "BLOCK-1",
						"fields": map[string]interface{}{
							"status": map[string]interface{}{"name": "In Progress"},
						},
					},
				},
			},
		},
	}
	blockers := client.GetBlockerKeys(ticket)
	assert.Contains(t, blockers, "BLOCK-1")

	// Case 2: Blocked by Done ticket (should be ignored)
	ticketDone := map[string]interface{}{
		"fields": map[string]interface{}{
			"issuelinks": []interface{}{
				map[string]interface{}{
					"type": map[string]interface{}{"inward": "is blocked by"},
					"inwardIssue": map[string]interface{}{
						"key": "BLOCK-2",
						"fields": map[string]interface{}{
							"status": map[string]interface{}{"name": "Done"},
						},
					},
				},
			},
		},
	}
	blockersDone := client.GetBlockerKeys(ticketDone)
	assert.Empty(t, blockersDone)

	// Case 3: Malformed ticket (nil fields)
	blockersNil := client.GetBlockerKeys(map[string]interface{}{})
	assert.Empty(t, blockersNil)
}

func TestClient_Authenticate_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	err := client.Authenticate(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "authentication failed with status: 401")
}

func TestClient_CreateTicket_Failure(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusInternalServerError)
        w.Write([]byte("Server Error"))
    }))
    defer server.Close()

    client := NewClient(server.URL, "user", "token")
    _, err := client.CreateTicket(context.Background(), "PROJ", "Summary", "Desc", "Task", nil)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "failed to create ticket with status: 500")
}

func TestClient_SmartTransition(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock transitions response
		if r.URL.Path == "/rest/api/3/issue/PROJ-1/transitions" {
			if r.Method == "GET" {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"transitions": [{"id": "11", "name": "In Progress"}, {"id": "21", "name": "Done"}]}`))
				return
			} else if r.Method == "POST" {
				var payload map[string]interface{}
				json.NewDecoder(r.Body).Decode(&payload)
				trans, _ := payload["transition"].(map[string]interface{})
				id, _ := trans["id"].(string)
				if id == "11" {
					w.WriteHeader(http.StatusNoContent)
					return
				}
				w.WriteHeader(http.StatusBadRequest)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")

	// Test by Name
	err := client.SmartTransition(context.Background(), "PROJ-1", "In Progress")
	assert.NoError(t, err)

	// Test by ID
	err = client.SmartTransition(context.Background(), "PROJ-1", "11")
	assert.NoError(t, err)

	// Test Not Found
	err = client.SmartTransition(context.Background(), "PROJ-1", "NonExistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no transition found matching")
}

func TestClient_SearchIssues(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/search/jql" {
			q := r.URL.Query()
			if q.Get("jql") == "project = PROJ" {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"issues": [{"key": "PROJ-1"}]}`))
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	issues, err := client.SearchIssues(context.Background(), "project = PROJ")
	assert.NoError(t, err)
	assert.Len(t, issues, 1)
	assert.Equal(t, "PROJ-1", issues[0]["key"])
}

func TestClient_GetBlockers(t *testing.T) {
	client := NewClient("http://jira.local", "user", "token")
	ticket := map[string]interface{}{
		"fields": map[string]interface{}{
			"issuelinks": []interface{}{
				map[string]interface{}{
					"type": map[string]interface{}{"inward": "is blocked by"},
					"inwardIssue": map[string]interface{}{
						"key": "BLOCK-1",
						"fields": map[string]interface{}{
							"status": map[string]interface{}{"name": "In Progress"},
						},
					},
				},
			},
		},
	}
	blockers := client.GetBlockers(ticket)
	assert.Len(t, blockers, 1)
	assert.Equal(t, "BLOCK-1 (In Progress)", blockers[0])
}

func TestClient_CreateChildTicket(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/issue" && r.Method == "POST" {
			var payload map[string]interface{}
			json.NewDecoder(r.Body).Decode(&payload)
			fields, _ := payload["fields"].(map[string]interface{})
			parent, _ := fields["parent"].(map[string]interface{})
			if parent["key"] == "PARENT-1" {
				w.WriteHeader(http.StatusCreated)
				w.Write([]byte(`{"key": "CHILD-1"}`))
				return
			}
		}
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	key, err := client.CreateChildTicket(context.Background(), "PROJ", "Child", "Desc", "Task", "PARENT-1", nil)
	assert.NoError(t, err)
	assert.Equal(t, "CHILD-1", key)
}
