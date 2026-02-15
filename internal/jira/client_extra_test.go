package jira

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateTicket_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue" || r.Method != "POST" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		var payload map[string]interface{}
		json.NewDecoder(r.Body).Decode(&payload)

		// Verify payload
		fields, ok := payload["fields"].(map[string]interface{})
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if fields["summary"] != "New Ticket" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"key": "NEW-1", "id": "10001"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	key, err := client.CreateTicket(context.Background(), "PROJ", "New Ticket", "Description", "Task", []string{"label"})
	if err != nil {
		t.Fatalf("CreateTicket failed: %v", err)
	}
	if key != "NEW-1" {
		t.Errorf("Expected key NEW-1, got %s", key)
	}
}

func TestSearchIssues_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/search/jql" || r.Method != "GET" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		jql := r.URL.Query().Get("jql")
		if jql != "project = PROJ" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"issues": [
				{"key": "PROJ-1", "fields": {"summary": "Issue 1"}},
				{"key": "PROJ-2", "fields": {"summary": "Issue 2"}}
			]
		}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	issues, err := client.SearchIssues(context.Background(), "project = PROJ")
	if err != nil {
		t.Fatalf("SearchIssues failed: %v", err)
	}
	if len(issues) != 2 {
		t.Errorf("Expected 2 issues, got %d", len(issues))
	}
}

func TestSmartTransition_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/issue/PROJ-1/transitions" {
			if r.Method == "GET" {
				w.Write([]byte(`{
					"transitions": [
						{"id": "11", "name": "In Progress"},
						{"id": "21", "name": "Done"}
					]
				}`))
				return
			} else if r.Method == "POST" {
				var payload map[string]interface{}
				json.NewDecoder(r.Body).Decode(&payload)
				trans := payload["transition"].(map[string]interface{})
				if trans["id"] == "11" {
					w.WriteHeader(http.StatusNoContent)
					return
				}
			}
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")

	// Test by Name
	err := client.SmartTransition(context.Background(), "PROJ-1", "In Progress")
	if err != nil {
		t.Errorf("SmartTransition by name failed: %v", err)
	}

	// Test by ID
	err = client.SmartTransition(context.Background(), "PROJ-1", "11")
	if err != nil {
		t.Errorf("SmartTransition by ID failed: %v", err)
	}

	// Test Not Found
	err = client.SmartTransition(context.Background(), "PROJ-1", "Unknown")
	if err == nil {
		t.Error("Expected error for unknown transition")
	}
}

func TestGetBlockers(t *testing.T) {
	ticket := map[string]interface{}{
		"fields": map[string]interface{}{
			"issuelinks": []interface{}{
				map[string]interface{}{
					"type": map[string]interface{}{"inward": "is blocked by"},
					"inwardIssue": map[string]interface{}{
						"key": "BLOCKER-1",
						"fields": map[string]interface{}{
							"status": map[string]interface{}{"name": "Open"},
						},
					},
				},
				map[string]interface{}{
					"type": map[string]interface{}{"inward": "is blocked by"},
					"inwardIssue": map[string]interface{}{
						"key": "BLOCKER-2",
						"fields": map[string]interface{}{
							"status": map[string]interface{}{"name": "Done"},
						},
					},
				},
				map[string]interface{}{
					"type": map[string]interface{}{"inward": "relates to"}, // Not a blocker
					"inwardIssue": map[string]interface{}{
						"key": "REL-1",
					},
				},
			},
		},
	}

	client := &Client{}

	blockerKeys := client.GetBlockerKeys(ticket)
	if len(blockerKeys) != 1 || blockerKeys[0] != "BLOCKER-1" {
		t.Errorf("Expected [BLOCKER-1], got %v", blockerKeys)
	}

	blockers := client.GetBlockers(ticket)
	if len(blockers) != 1 || blockers[0] != "BLOCKER-1 (Open)" {
		t.Errorf("Expected [BLOCKER-1 (Open)], got %v", blockers)
	}
}

func TestLoadLabelIssues_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/search/jql" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		jql := r.URL.Query().Get("jql")
		if jql != "labels = \"test-label\"" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.Write([]byte(`{
			"issues": [
				{"key": "LABEL-1"}
			]
		}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	issues, err := client.LoadLabelIssues(context.Background(), "test-label")
	if err != nil {
		t.Fatalf("LoadLabelIssues failed: %v", err)
	}
	if len(issues) != 1 || issues[0]["key"] != "LABEL-1" {
		t.Errorf("Expected issue LABEL-1, got %v", issues)
	}
}

func TestCreateTicket_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	_, err := client.CreateTicket(context.Background(), "PROJ", "Sum", "Desc", "Task", nil)
	if err == nil {
		t.Error("Expected error for CreateTicket failure")
	}
}

func TestCreateChildTicket_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		var payload map[string]interface{}
		json.NewDecoder(r.Body).Decode(&payload)
		fields := payload["fields"].(map[string]interface{})

		// Check parent
		parent, ok := fields["parent"].(map[string]interface{})
		if !ok || parent["key"] != "PARENT-1" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"key": "CHILD-1"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	key, err := client.CreateChildTicket(context.Background(), "PROJ", "Child", "Desc", "Sub-task", "PARENT-1", nil)
	if err != nil {
		t.Fatalf("CreateChildTicket failed: %v", err)
	}
	if key != "CHILD-1" {
		t.Errorf("Expected CHILD-1, got %s", key)
	}
}

func TestGetTransitions_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	_, err := client.GetTransitions(context.Background(), "PROJ-1")
	if err == nil {
		t.Error("Expected error for GetTransitions failure")
	}
}

func TestDeleteIssue_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue/PROJ-1" || r.Method != "DELETE" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	err := client.DeleteIssue(context.Background(), "PROJ-1")
	if err != nil {
		t.Fatalf("DeleteIssue failed: %v", err)
	}
}
