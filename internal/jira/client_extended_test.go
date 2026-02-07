package jira

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetBlockerKeys(t *testing.T) {
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
				map[string]interface{}{
					"type": map[string]interface{}{
						"inward": "relates to", // Should be ignored
					},
					"inwardIssue": map[string]interface{}{
						"key": "REL-1",
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
								"name": "Done", // Should be ignored as it is done
							},
						},
					},
				},
			},
		},
	}

	blockers := client.GetBlockerKeys(ticket)

	if len(blockers) != 1 {
		t.Fatalf("Expected 1 blocker, got %d", len(blockers))
	}

	if blockers[0] != "BLOCK-1" {
		t.Errorf("Expected blocker key BLOCK-1, got %s", blockers[0])
	}
}

func TestCreateTicket_WithLabels(t *testing.T) {
	var receivedPayload map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		json.NewDecoder(r.Body).Decode(&receivedPayload)

		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id": "10000", "key": "PROJ-101", "self": "http://jira.local/rest/api/3/issue/10000"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	key, err := client.CreateTicket(context.Background(), "PROJ", "Summary", "Description", "Task", []string{"label1"})

	if err != nil {
		t.Fatalf("CreateTicket failed: %v", err)
	}

	if key != "PROJ-101" {
		t.Errorf("Expected key PROJ-101, got %s", key)
	}

	// Verify payload
	fields, ok := receivedPayload["fields"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected fields in payload")
	}

	if fields["summary"] != "Summary" {
		t.Errorf("Expected summary 'Summary', got %v", fields["summary"])
	}

	project, ok := fields["project"].(map[string]interface{})
	if !ok || project["key"] != "PROJ" {
		t.Errorf("Expected project key 'PROJ', got %v", project)
	}
}

func TestTransitionIssue_Errors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"errorMessages":["Invalid transition"]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	err := client.TransitionIssue(context.Background(), "PROJ-123", "31")

	if err == nil {
		t.Fatal("Expected error for bad request")
	}
}

func TestAddIssueLink_Success(t *testing.T) {
	var receivedPayload map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issueLink" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		json.NewDecoder(r.Body).Decode(&receivedPayload)
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	err := client.AddIssueLink(context.Background(), "PROJ-1", "PROJ-2", "Blocks")

	if err != nil {
		t.Fatalf("AddIssueLink failed: %v", err)
	}

	typeMap, ok := receivedPayload["type"].(map[string]interface{})
	if !ok || typeMap["name"] != "Blocks" {
		t.Errorf("Expected link type 'Blocks', got %v", typeMap)
	}
}

func TestSetParent_Success(t *testing.T) {
	var receivedPayload map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue/SUB-1" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method != "PUT" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		json.NewDecoder(r.Body).Decode(&receivedPayload)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	err := client.SetParent(context.Background(), "SUB-1", "PARENT-1")

	if err != nil {
		t.Fatalf("SetParent failed: %v", err)
	}

	fields, ok := receivedPayload["fields"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected fields in payload")
	}

	parent, ok := fields["parent"].(map[string]interface{})
	if !ok || parent["key"] != "PARENT-1" {
		t.Errorf("Expected parent key 'PARENT-1', got %v", parent)
	}
}

func TestAddLabel_Success(t *testing.T) {
	var receivedPayload map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue/PROJ-1" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method != "PUT" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		json.NewDecoder(r.Body).Decode(&receivedPayload)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	err := client.AddLabel(context.Background(), "PROJ-1", "new-label")

	if err != nil {
		t.Fatalf("AddLabel failed: %v", err)
	}

	update, ok := receivedPayload["update"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected update in payload")
	}

	labels, ok := update["labels"].([]interface{})
	if !ok || len(labels) == 0 {
		t.Fatal("Expected labels update")
	}

	addOp, ok := labels[0].(map[string]interface{})
	if !ok || addOp["add"] != "new-label" {
		t.Errorf("Expected add operation for 'new-label', got %v", addOp)
	}
}
