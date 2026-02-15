package jira

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetBlockerKeys(t *testing.T) {
	client := NewClient("http://jira.local", "user", "token")

	// Construct a ticket structure manually
	ticket := map[string]interface{}{
		"fields": map[string]interface{}{
			"issuelinks": []interface{}{
				// Not a blocker (wrong type)
				map[string]interface{}{
					"type": map[string]interface{}{
						"inward": "relates to",
					},
				},
				// Blocker, but done
				map[string]interface{}{
					"type": map[string]interface{}{
						"inward": "is blocked by",
					},
					"inwardIssue": map[string]interface{}{
						"key": "DONE-1",
						"fields": map[string]interface{}{
							"status": map[string]interface{}{
								"name": "Done",
							},
						},
					},
				},
				// Blocker, not done (should be returned)
				map[string]interface{}{
					"type": map[string]interface{}{
						"inward": "is blocked by",
					},
					"inwardIssue": map[string]interface{}{
						"key": "BLOCK-1",
						"fields": map[string]interface{}{
							"status": map[string]interface{}{
								"name": "In Progress",
							},
						},
					},
				},
				// Blocker, another not done (should be returned)
				map[string]interface{}{
					"type": map[string]interface{}{
						"inward": "is blocked by",
					},
					"inwardIssue": map[string]interface{}{
						"key": "BLOCK-2",
						"fields": map[string]interface{}{
							"status": map[string]interface{}{
								"name": "Open",
							},
						},
					},
				},
			},
		},
	}

	blockers := client.GetBlockerKeys(ticket)

	if len(blockers) != 2 {
		t.Fatalf("Expected 2 blockers, got %d", len(blockers))
	}

	foundBlock1 := false
	foundBlock2 := false
	for _, key := range blockers {
		if key == "BLOCK-1" {
			foundBlock1 = true
		}
		if key == "BLOCK-2" {
			foundBlock2 = true
		}
	}

	if !foundBlock1 || !foundBlock2 {
		t.Errorf("Expected BLOCK-1 and BLOCK-2, got %v", blockers)
	}
}

func TestGetBlockerKeys_InvalidFormat(t *testing.T) {
	client := NewClient("http://jira.local", "user", "token")

	// Missing fields
	ticket := map[string]interface{}{}
	blockers := client.GetBlockerKeys(ticket)
	if blockers != nil {
		t.Error("Expected nil blockers for missing fields")
	}

	// Missing issuelinks
	ticket = map[string]interface{}{
		"fields": map[string]interface{}{},
	}
	blockers = client.GetBlockerKeys(ticket)
	if blockers != nil {
		t.Error("Expected nil blockers for missing issuelinks")
	}
}

func TestCreateChildTicket_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		// Verify payload contains parent
		var payload map[string]interface{}
		json.NewDecoder(r.Body).Decode(&payload)

		fields, ok := payload["fields"].(map[string]interface{})
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		parent, ok := fields["parent"].(map[string]interface{})
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if parent["key"] != "PARENT-1" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"key": "CHILD-1"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	key, err := client.CreateChildTicket(context.Background(), "PROJ", "Summary", "Desc", "Sub-task", "PARENT-1", nil)
	if err != nil {
		t.Fatalf("CreateChildTicket failed: %v", err)
	}

	if key != "CHILD-1" {
		t.Errorf("Expected CHILD-1, got %s", key)
	}
}

func TestCreateChildTicket_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "bad request"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	_, err := client.CreateChildTicket(context.Background(), "PROJ", "Summary", "Desc", "Sub-task", "PARENT-1", nil)
	if err == nil {
		t.Error("Expected error for bad request")
	}
}

func TestSmartTransition_NoMatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/issue/TICKET-1/transitions" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"transitions": [{"id": "11", "name": "Start"}]}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	err := client.SmartTransition(context.Background(), "TICKET-1", "Finish")
	if err == nil {
		t.Error("Expected error for non-matching transition")
	}
}

func TestSmartTransition_FetchError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	err := client.SmartTransition(context.Background(), "TICKET-1", "Start")
	if err == nil {
		t.Error("Expected error when fetch fails")
	}
}

func TestCreateTicket_Success(t *testing.T) {
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
		w.Write([]byte(`{"key": "NEW-1"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	key, err := client.CreateTicket(context.Background(), "PROJ", "New Ticket", "Desc", "Task", nil)
	if err != nil {
		t.Fatalf("CreateTicket failed: %v", err)
	}

	if key != "NEW-1" {
		t.Errorf("Expected NEW-1, got %s", key)
	}
}

func TestSearchIssues_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/search/jql" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		q := r.URL.Query()
		if q.Get("jql") != "project = PROJ" {
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

func TestLoadLabelIssues_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/search/jql" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		q := r.URL.Query()
		if !strings.Contains(q.Get("jql"), "labels = \"mylabel\"") {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"issues": [{"key": "L-1"}]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	issues, err := client.LoadLabelIssues(context.Background(), "mylabel")
	if err != nil {
		t.Fatalf("LoadLabelIssues failed: %v", err)
	}

	if len(issues) != 1 || issues[0]["key"] != "L-1" {
		t.Errorf("Expected L-1, got %v", issues)
	}
}

func TestGetBlockers(t *testing.T) {
	client := NewClient("http://jira.local", "user", "token")

	// Construct a ticket with blockers
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
			},
		},
	}

	blockers := client.GetBlockers(ticket)
	if len(blockers) != 1 {
		t.Fatalf("Expected 1 blocker, got %d", len(blockers))
	}
	if blockers[0] != "BLOCK-1 (Open)" {
		t.Errorf("Expected 'BLOCK-1 (Open)', got %s", blockers[0])
	}
}

func TestDeleteIssue_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/issue/DEL-1" && r.Method == "DELETE" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	err := client.DeleteIssue(context.Background(), "DEL-1")
	if err != nil {
		t.Fatalf("DeleteIssue failed: %v", err)
	}
}
