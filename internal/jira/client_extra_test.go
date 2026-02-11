package jira

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCreateTicket_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/rest/api/3/issue" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var payload map[string]interface{}
		json.NewDecoder(r.Body).Decode(&payload)

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
		w.Write([]byte(`{"key": "PROJ-100"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	key, err := client.CreateTicket(context.Background(), "PROJ", "New Ticket", "Description", "Story", []string{"label"})
	if err != nil {
		t.Fatalf("CreateTicket failed: %v", err)
	}
	if key != "PROJ-100" {
		t.Errorf("Expected key PROJ-100, got %s", key)
	}
}

func TestSearchIssues_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/search/jql" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		query := r.URL.Query().Get("jql")
		if query == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"issues": [{"key": "PROJ-1"}, {"key": "PROJ-2"}]}`))
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

func TestLoadLabelIssues(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jql := r.URL.Query().Get("jql")
		if !strings.Contains(jql, "labels = \"my-label\"") {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"issues": [{"key": "PROJ-1"}]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	issues, err := client.LoadLabelIssues(context.Background(), "my-label")
	if err != nil {
		t.Fatalf("LoadLabelIssues failed: %v", err)
	}
	if len(issues) != 1 {
		t.Errorf("Expected 1 issue, got %d", len(issues))
	}
}

func TestGetTransitions_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue/PROJ-1/transitions" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"transitions": [{"id": "11", "name": "In Progress"}, {"id": "21", "name": "Done"}]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	transitions, err := client.GetTransitions(context.Background(), "PROJ-1")
	if err != nil {
		t.Fatalf("GetTransitions failed: %v", err)
	}
	if len(transitions) != 2 {
		t.Errorf("Expected 2 transitions, got %d", len(transitions))
	}
}

func TestSmartTransition_ByName(t *testing.T) {
	var transitionID string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"transitions": [{"id": "11", "name": "Start Progress"}]}`))
			return
		}
		if r.Method == "POST" {
			var payload map[string]interface{}
			json.NewDecoder(r.Body).Decode(&payload)
			transitionID = payload["transition"].(map[string]interface{})["id"].(string)
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	err := client.SmartTransition(context.Background(), "PROJ-1", "Start Progress")
	if err != nil {
		t.Fatalf("SmartTransition failed: %v", err)
	}
	if transitionID != "11" {
		t.Errorf("Expected transition ID 11, got %s", transitionID)
	}
}

func TestSmartTransition_ByID(t *testing.T) {
	var transitionID string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"transitions": [{"id": "11", "name": "Start Progress"}]}`))
			return
		}
		if r.Method == "POST" {
			var payload map[string]interface{}
			json.NewDecoder(r.Body).Decode(&payload)
			transitionID = payload["transition"].(map[string]interface{})["id"].(string)
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	err := client.SmartTransition(context.Background(), "PROJ-1", "11")
	if err != nil {
		t.Fatalf("SmartTransition failed: %v", err)
	}
	if transitionID != "11" {
		t.Errorf("Expected transition ID 11, got %s", transitionID)
	}
}

func TestSmartTransition_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"transitions": [{"id": "11", "name": "Start Progress"}]}`))
			return
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	err := client.SmartTransition(context.Background(), "PROJ-1", "Done")
	if err == nil {
		t.Error("Expected error for non-existent transition")
	}
}

func TestGetBlockerKeys(t *testing.T) {
	client := NewClient("http://jira.local", "user", "token")

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
								"name": "To Do",
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
				map[string]interface{}{
					"type": map[string]interface{}{
						"inward": "relates to",
					},
				},
			},
		},
	}

	keys := client.GetBlockerKeys(ticket)
	if len(keys) != 1 {
		t.Fatalf("Expected 1 blocker, got %d", len(keys))
	}
	if keys[0] != "BLOCK-1" {
		t.Errorf("Expected blocker BLOCK-1, got %s", keys[0])
	}
}

func TestGetBlockers(t *testing.T) {
	client := NewClient("http://jira.local", "user", "token")

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
								"name": "To Do",
							},
						},
					},
				},
			},
		},
	}

	blockers := client.GetBlockers(ticket)
	if len(blockers) != 1 {
		t.Fatalf("Expected 1 blocker, got %d", len(blockers))
	}
	expected := "BLOCK-1 (To Do)"
	if blockers[0] != expected {
		t.Errorf("Expected %s, got %s", expected, blockers[0])
	}
}

func TestDeleteIssue_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" || r.URL.Path != "/rest/api/3/issue/PROJ-1" {
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

func TestGetTicket_Error500(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	_, err := client.GetTicket(context.Background(), "PROJ-1")
	if err == nil {
		t.Error("Expected error for 500")
	}
}

func TestCreateChildTicket_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/rest/api/3/issue" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var payload map[string]interface{}
		json.NewDecoder(r.Body).Decode(&payload)

		fields, ok := payload["fields"].(map[string]interface{})
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if fields["summary"] != "Child Ticket" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		parent, ok := fields["parent"].(map[string]interface{})
		if !ok || parent["key"] != "PARENT-1" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"key": "PROJ-101"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	key, err := client.CreateChildTicket(context.Background(), "PROJ", "Child Ticket", "Desc", "Subtask", "PARENT-1", nil)
	if err != nil {
		t.Fatalf("CreateChildTicket failed: %v", err)
	}
	if key != "PROJ-101" {
		t.Errorf("Expected key PROJ-101, got %s", key)
	}
}

func TestCreateTicket_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"errorMessages":["Bad Request"]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	_, err := client.CreateTicket(context.Background(), "PROJ", "Summary", "Desc", "Story", nil)
	if err == nil {
		t.Error("Expected error for 400")
	}
}

func TestSearchIssues_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	_, err := client.SearchIssues(context.Background(), "project = PROJ")
	if err == nil {
		t.Error("Expected error for 500")
	}
}
