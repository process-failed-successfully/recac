package jira

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClient_CreateTicket(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var payload map[string]interface{}
		json.NewDecoder(r.Body).Decode(&payload)

		fields := payload["fields"].(map[string]interface{})
		if fields["project"].(map[string]interface{})["key"] != "TEST" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"key": "TEST-1"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	key, err := client.CreateTicket(context.Background(), "TEST", "Summary", "Desc", "Task", []string{"label"})
	if err != nil {
		t.Fatalf("CreateTicket failed: %v", err)
	}
	if key != "TEST-1" {
		t.Errorf("Expected key TEST-1, got %s", key)
	}
}

func TestClient_SearchIssues(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/search/jql" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method != "GET" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		q := r.URL.Query().Get("jql")
		if q != "project = TEST" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"issues": [{"key": "TEST-1"}]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	issues, err := client.SearchIssues(context.Background(), "project = TEST")
	if err != nil {
		t.Fatalf("SearchIssues failed: %v", err)
	}
	if len(issues) != 1 {
		t.Errorf("Expected 1 issue, got %d", len(issues))
	}
	if issues[0]["key"] != "TEST-1" {
		t.Errorf("Expected issue TEST-1, got %v", issues[0]["key"])
	}
}

func TestClient_LoadLabelIssues(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("jql")
		if q != "labels = \"mylabel\"" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"issues": [{"key": "TEST-2"}]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	issues, err := client.LoadLabelIssues(context.Background(), "mylabel")
	if err != nil {
		t.Fatalf("LoadLabelIssues failed: %v", err)
	}
	if len(issues) != 1 || issues[0]["key"] != "TEST-2" {
		t.Errorf("Unexpected result: %v", issues)
	}
}

func TestClient_GetTransitions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue/TEST-1/transitions" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"transitions": [{"id": "1", "name": "Done"}]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	transitions, err := client.GetTransitions(context.Background(), "TEST-1")
	if err != nil {
		t.Fatalf("GetTransitions failed: %v", err)
	}
	if len(transitions) != 1 {
		t.Errorf("Expected 1 transition, got %d", len(transitions))
	}
	if transitions[0]["name"] != "Done" {
		t.Errorf("Expected transition Done, got %v", transitions[0]["name"])
	}
}

func TestClient_SmartTransition(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/issue/TEST-1/transitions" {
			if r.Method == "GET" {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"transitions": [{"id": "10", "name": "In Progress"}]}`))
				return
			} else if r.Method == "POST" {
				var payload map[string]interface{}
				json.NewDecoder(r.Body).Decode(&payload)
				id := payload["transition"].(map[string]interface{})["id"]
				if id == "10" {
					w.WriteHeader(http.StatusNoContent)
					return
				}
			}
		}
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")

	// Test by Name
	err := client.SmartTransition(context.Background(), "TEST-1", "In Progress")
	if err != nil {
		t.Errorf("SmartTransition by name failed: %v", err)
	}

	// Test by ID
	err = client.SmartTransition(context.Background(), "TEST-1", "10")
	if err != nil {
		t.Errorf("SmartTransition by ID failed: %v", err)
	}

	// Test Fail
	err = client.SmartTransition(context.Background(), "TEST-1", "Unknown")
	if err == nil {
		t.Error("Expected error for unknown transition")
	}
}

func TestClient_GetBlockers(t *testing.T) {
	ticket := map[string]interface{}{
		"fields": map[string]interface{}{
			"issuelinks": []interface{}{
				map[string]interface{}{
					"type": map[string]interface{}{
						"inward": "is blocked by",
					},
					"inwardIssue": map[string]interface{}{
						"key": "BLOCK-1",
						"fields": map[string]interface{}{
							"status": map[string]interface{}{
								"name": "Open",
							},
						},
					},
				},
				map[string]interface{}{
					"type": map[string]interface{}{
						"inward": "is blocked by",
					},
					"inwardIssue": map[string]interface{}{
						"key": "BLOCK-2",
						"fields": map[string]interface{}{
							"status": map[string]interface{}{
								"name": "Done",
							},
						},
					},
				},
			},
		},
	}

	client := &Client{} // Methods are pure logic on ticket map

	// Test GetBlockerKeys
	keys := client.GetBlockerKeys(ticket)
	if len(keys) != 1 || keys[0] != "BLOCK-1" {
		t.Errorf("Expected [BLOCK-1], got %v", keys)
	}

	// Test GetBlockers (formatted)
	blockers := client.GetBlockers(ticket)
	if len(blockers) != 1 || !strings.Contains(blockers[0], "BLOCK-1 (Open)") {
		t.Errorf("Expected BLOCK-1 (Open), got %v", blockers)
	}
}

func TestClient_AddIssueLink(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issueLink" || r.Method != "POST" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	err := client.AddIssueLink(context.Background(), "A", "B", "Blocks")
	if err != nil {
		t.Fatalf("AddIssueLink failed: %v", err)
	}
}

func TestClient_SetParent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue/CHILD-1" || r.Method != "PUT" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	err := client.SetParent(context.Background(), "CHILD-1", "PARENT-1")
	if err != nil {
		t.Fatalf("SetParent failed: %v", err)
	}
}

func TestClient_AddLabel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue/TEST-1" || r.Method != "PUT" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	err := client.AddLabel(context.Background(), "TEST-1", "new-label")
	if err != nil {
		t.Fatalf("AddLabel failed: %v", err)
	}
}

func TestClient_GetFirstProjectKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/project" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"key": "PROJ1"}, {"key": "PROJ2"}]`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	key, err := client.GetFirstProjectKey(context.Background())
	if err != nil {
		t.Fatalf("GetFirstProjectKey failed: %v", err)
	}
	if key != "PROJ1" {
		t.Errorf("Expected PROJ1, got %s", key)
	}
}

func TestClient_DeleteIssue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue/DEL-1" || r.Method != "DELETE" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	err := client.DeleteIssue(context.Background(), "DEL-1")
	if err != nil {
		t.Fatalf("DeleteIssue failed: %v", err)
	}
}

func TestClient_GetFirstProjectKey_Empty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	_, err := client.GetFirstProjectKey(context.Background())
	if err == nil {
		t.Error("Expected error for no projects")
	}
}

func TestClient_CreateTicket_Fail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	_, err := client.CreateTicket(context.Background(), "P", "S", "D", "T", nil)
	if err == nil {
		t.Error("Expected error on server failure")
	}
}
