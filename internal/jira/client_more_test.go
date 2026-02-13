package jira

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetBlockerKeys(t *testing.T) {
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
			expected: []string{"RD-158"},
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
			blockers := client.GetBlockerKeys(tt.ticket)
			if len(blockers) != len(tt.expected) {
				t.Errorf("expected %d blockers, got %d", len(tt.expected), len(blockers))
			}
			for i, b := range blockers {
				if b != tt.expected[i] {
					t.Errorf("expected blocker key %q, got %q", tt.expected[i], b)
				}
			}
		})
	}
}

func TestAddIssueLink_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"errorMessages":["Link type does not exist"]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	err := client.AddIssueLink(context.Background(), "A-1", "B-1", "InvalidType")
	if err == nil {
		t.Error("Expected error for invalid link type")
	}
	if !strings.Contains(err.Error(), "Link type does not exist") {
		t.Errorf("Expected error message to contain 'Link type does not exist', got %v", err)
	}
}

func TestSetParent_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"errorMessages":["Issue not found"]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	err := client.SetParent(context.Background(), "SUB-1", "PARENT-999")
	if err == nil {
		t.Error("Expected error for non-existent parent")
	}
	if !strings.Contains(err.Error(), "Issue not found") {
		t.Errorf("Expected error message to contain 'Issue not found', got %v", err)
	}
}

func TestAddLabel_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	err := client.AddLabel(context.Background(), "PROJ-1", "new-label")
	if err == nil {
		t.Error("Expected error for server error")
	}
}

func TestCreateTicket_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Invalid Payload"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	_, err := client.CreateTicket(context.Background(), "PROJ", "Summary", "Desc", "Task", nil)
	if err == nil {
		t.Error("Expected error for bad request")
	}
}

func TestCreateTicket_DecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	_, err := client.CreateTicket(context.Background(), "PROJ", "Summary", "Desc", "Task", nil)
	if err == nil {
		t.Error("Expected error for invalid JSON response")
	}
	if !strings.Contains(err.Error(), "failed to decode response") {
		t.Errorf("Expected error message about decoding, got %v", err)
	}
}

func TestSearchIssues_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	_, err := client.SearchIssues(context.Background(), "invalid jql")
	if err == nil {
		t.Error("Expected error for invalid JQL")
	}
}

func TestSmartTransition_NoTransitions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock empty transitions list
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"transitions": []}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	err := client.SmartTransition(context.Background(), "PROJ-1", "Done")
	if err == nil {
		t.Error("Expected error when no transitions found")
	}
	if !strings.Contains(err.Error(), "no transition found") {
		t.Errorf("Expected error message 'no transition found', got %v", err)
	}
}

func TestCreateChildTicket_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	_, err := client.CreateChildTicket(context.Background(), "PROJ", "Summary", "Desc", "Subtask", "PARENT-1", nil)
	if err == nil {
		t.Error("Expected error for bad request")
	}
}

func TestCreateChildTicket_DecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	_, err := client.CreateChildTicket(context.Background(), "PROJ", "Summary", "Desc", "Subtask", "PARENT-1", nil)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestAuthenticate_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	err := client.Authenticate(context.Background())
	if err == nil {
		t.Error("Expected error for unauthorized")
	}
}

func TestGetTicket_DecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	_, err := client.GetTicket(context.Background(), "PROJ-1")
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestGetTransitions_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	_, err := client.GetTransitions(context.Background(), "PROJ-1")
	if err == nil {
		t.Error("Expected error for not found ticket")
	}
}

func TestGetTransitions_DecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	_, err := client.GetTransitions(context.Background(), "PROJ-1")
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestSmartTransition_FetchError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	err := client.SmartTransition(context.Background(), "PROJ-1", "Done")
	if err == nil {
		t.Error("Expected error for fetch failure")
	}
}

func TestGetFirstProjectKey_DecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	_, err := client.GetFirstProjectKey(context.Background())
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestGetFirstProjectKey_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	_, err := client.GetFirstProjectKey(context.Background())
	if err == nil {
		t.Error("Expected error for server error")
	}
}

func TestGetFirstProjectKey_InvalidFormat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id": 123}]`)) // Missing key
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	_, err := client.GetFirstProjectKey(context.Background())
	if err == nil {
		t.Error("Expected error for missing key")
	}
}

func TestParseDescription_Empty(t *testing.T) {
	client := NewClient("", "", "")
	desc := client.ParseDescription(map[string]interface{}{})
	if desc != "" {
		t.Errorf("Expected empty description, got %q", desc)
	}
}

func TestParseDescription_InvalidFormat(t *testing.T) {
	client := NewClient("", "", "")
	desc := client.ParseDescription(map[string]interface{}{
		"fields": map[string]interface{}{
			"description": "Just a string", // Expected map
		},
	})
	if desc != "" {
		t.Errorf("Expected empty description, got %q", desc)
	}
}
