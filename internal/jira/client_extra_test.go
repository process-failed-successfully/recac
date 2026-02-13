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
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("{\"key\": \"PROJ-101\"}"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	key, err := client.CreateTicket(context.Background(), "PROJ", "Summary", "Desc", "Task", nil)
	if err != nil {
		t.Fatalf("CreateTicket failed: %v", err)
	}
	if key != "PROJ-101" {
		t.Errorf("Expected key PROJ-101, got %s", key)
	}
}

func TestDeleteIssue_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue/PROJ-123" || r.Method != "DELETE" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	if err := client.DeleteIssue(context.Background(), "PROJ-123"); err != nil {
		t.Fatalf("DeleteIssue failed: %v", err)
	}
}

func TestSearchIssues_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/search/jql" || r.Method != "GET" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{\"issues\": [{\"key\": \"PROJ-123\"}]}"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	issues, err := client.SearchIssues(context.Background(), "project = PROJ")
	if err != nil {
		t.Fatalf("SearchIssues failed: %v", err)
	}
	if len(issues) != 1 || issues[0]["key"] != "PROJ-123" {
		t.Error("SearchIssues returned incorrect data")
	}
}

func TestLoadLabelIssues_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/search/jql" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		q := r.URL.Query().Get("jql")
		if q != "labels = \"mylabel\"" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{\"issues\": [{\"key\": \"PROJ-123\"}]}"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	issues, err := client.LoadLabelIssues(context.Background(), "mylabel")
	if err != nil {
		t.Fatalf("LoadLabelIssues failed: %v", err)
	}
	if len(issues) != 1 {
		t.Error("LoadLabelIssues returned incorrect data")
	}
}

func TestGetTransitions_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue/PROJ-123/transitions" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{\"transitions\": [{\"id\": \"31\", \"name\": \"Done\"}]}"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	trans, err := client.GetTransitions(context.Background(), "PROJ-123")
	if err != nil {
		t.Fatalf("GetTransitions failed: %v", err)
	}
	if len(trans) != 1 || trans[0]["id"] != "31" {
		t.Error("GetTransitions returned incorrect data")
	}
}

func TestSmartTransition_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/issue/PROJ-123/transitions" {
			if r.Method == "GET" {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("{\"transitions\": [{\"id\": \"31\", \"name\": \"Done\"}]}"))
				return
			}
			if r.Method == "POST" {
				var payload map[string]interface{}
				json.NewDecoder(r.Body).Decode(&payload)
				if payload["transition"].(map[string]interface{})["id"] == "31" {
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
	if err := client.SmartTransition(context.Background(), "PROJ-123", "Done"); err != nil {
		t.Errorf("SmartTransition by name failed: %v", err)
	}

	// Test by ID
	if err := client.SmartTransition(context.Background(), "PROJ-123", "31"); err != nil {
		t.Errorf("SmartTransition by ID failed: %v", err)
	}

	// Test Invalid
	if err := client.SmartTransition(context.Background(), "PROJ-123", "Invalid"); err == nil {
		t.Error("SmartTransition expected error for invalid transition")
	}
}

func TestCreateChildTicket_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		// Verify payload has parent
		var payload map[string]interface{}
		json.NewDecoder(r.Body).Decode(&payload)
		fields := payload["fields"].(map[string]interface{})
		parent := fields["parent"].(map[string]interface{})
		if parent["key"] != "PARENT-1" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("{\"key\": \"CHILD-1\"}"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	key, err := client.CreateChildTicket(context.Background(), "PROJ", "Child", "Desc", "Sub-task", "PARENT-1", nil)
	if err != nil {
		t.Fatalf("CreateChildTicket failed: %v", err)
	}
	if key != "CHILD-1" {
		t.Errorf("Expected key CHILD-1, got %s", key)
	}
}

func TestGetBlockers(t *testing.T) {
	client := NewClient("", "", "")

	tests := []struct {
		name     string
		ticket   map[string]interface{}
		expected []string
	}{
		{
			name: "No links",
			ticket: map[string]interface{}{
				"fields": map[string]interface{}{},
			},
			expected: nil,
		},
		{
			name: "No blockers",
			ticket: map[string]interface{}{
				"fields": map[string]interface{}{
					"issuelinks": []interface{}{
						map[string]interface{}{
							"type": map[string]interface{}{
								"inward": "relates to",
							},
						},
					},
				},
			},
			expected: nil,
		},
		{
			name: "Unresolved blocker",
			ticket: map[string]interface{}{
				"fields": map[string]interface{}{
					"issuelinks": []interface{}{
						map[string]interface{}{
							"type": map[string]interface{}{
								"inward": "is blocked by",
							},
							"inwardIssue": map[string]interface{}{
								"key": "RD-158",
								"fields": map[string]interface{}{
									"status": map[string]interface{}{
										"name": "In Progress",
									},
								},
							},
						},
					},
				},
			},
			expected: []string{"RD-158 (In Progress)"},
		},
		{
			name: "Resolved blocker",
			ticket: map[string]interface{}{
				"fields": map[string]interface{}{
					"issuelinks": []interface{}{
						map[string]interface{}{
							"type": map[string]interface{}{
								"inward": "is blocked by",
							},
							"inwardIssue": map[string]interface{}{
								"key": "RD-159",
								"fields": map[string]interface{}{
									"status": map[string]interface{}{
										"name": "Done",
									},
								},
							},
						},
					},
				},
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blockers := client.GetBlockers(tt.ticket)
			if len(blockers) != len(tt.expected) {
				t.Errorf("expected %d blockers, got %d", len(tt.expected), len(blockers))
			}
			for i, b := range blockers {
				if b != tt.expected[i] {
					t.Errorf("expected blocker %q, got %q", tt.expected[i], b)
				}
			}
		})
	}
}
