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

func TestCreateTicket_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Bad Request"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	_, err := client.CreateTicket(context.Background(), "PROJ", "Summary", "Desc", "Task", nil)
	if err == nil {
		t.Fatal("Expected error for bad request")
	}
	if !strings.Contains(err.Error(), "failed to create ticket with status: 400") {
		t.Errorf("Expected status 400 error, got %v", err)
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

func TestDeleteIssue_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("Forbidden"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	if err := client.DeleteIssue(context.Background(), "PROJ-123"); err == nil {
		t.Fatal("Expected error for forbidden request")
	} else if !strings.Contains(err.Error(), "failed to delete issue with status: 403") {
		t.Errorf("Expected status 403 error, got %v", err)
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

func TestSearchIssues_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	_, err := client.SearchIssues(context.Background(), "project = PROJ")
	if err == nil {
		t.Fatal("Expected error for internal server error")
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

func TestGetTransitions_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	_, err := client.GetTransitions(context.Background(), "PROJ-123")
	if err == nil {
		t.Fatal("Expected error for not found")
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

func TestSmartTransition_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	if err := client.SmartTransition(context.Background(), "PROJ-123", "Done"); err == nil {
		t.Fatal("Expected error for internal server error")
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

func TestCreateChildTicket_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Bad Request"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	_, err := client.CreateChildTicket(context.Background(), "PROJ", "Child", "Desc", "Sub-task", "PARENT-1", nil)
	if err == nil {
		t.Fatal("Expected error for bad request")
	}
	if !strings.Contains(err.Error(), "failed to create child ticket with status: 400") {
		t.Errorf("Expected status 400 error, got %v", err)
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

func TestGetBlockerKeys(t *testing.T) {
	client := NewClient("", "", "")

	tests := []struct {
		name     string
		ticket   map[string]interface{}
		expected []string
	}{
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

func TestAddIssueLink_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issueLink" || r.Method != "POST" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	if err := client.AddIssueLink(context.Background(), "IN-1", "OUT-1", "Blocks"); err != nil {
		t.Fatalf("AddIssueLink failed: %v", err)
	}
}

func TestAddIssueLink_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Bad Request"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	if err := client.AddIssueLink(context.Background(), "IN-1", "OUT-1", "Blocks"); err == nil {
		t.Fatal("Expected error for bad request")
	}
}

func TestSetParent_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue/CHILD-1" || r.Method != "PUT" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var payload map[string]interface{}
		json.NewDecoder(r.Body).Decode(&payload)
		fields := payload["fields"].(map[string]interface{})
		parent := fields["parent"].(map[string]interface{})
		if parent["key"] != "PARENT-1" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	if err := client.SetParent(context.Background(), "CHILD-1", "PARENT-1"); err != nil {
		t.Fatalf("SetParent failed: %v", err)
	}
}

func TestSetParent_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Bad Request"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	if err := client.SetParent(context.Background(), "CHILD-1", "PARENT-1"); err == nil {
		t.Fatal("Expected error for bad request")
	}
}

func TestAddLabel_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue/PROJ-1" || r.Method != "PUT" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var payload map[string]interface{}
		json.NewDecoder(r.Body).Decode(&payload)
		update := payload["update"].(map[string]interface{})
		labels := update["labels"].([]interface{})
		add := labels[0].(map[string]interface{})["add"]
		if add != "new-label" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	if err := client.AddLabel(context.Background(), "PROJ-1", "new-label"); err != nil {
		t.Fatalf("AddLabel failed: %v", err)
	}
}

func TestAddLabel_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Bad Request"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	if err := client.AddLabel(context.Background(), "PROJ-1", "new-label"); err == nil {
		t.Fatal("Expected error for bad request")
	}
}

func TestGetFirstProjectKey_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/project" || r.Method != "GET" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("[{\"key\": \"PROJ\"}]"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	key, err := client.GetFirstProjectKey(context.Background())
	if err != nil {
		t.Fatalf("GetFirstProjectKey failed: %v", err)
	}
	if key != "PROJ" {
		t.Errorf("Expected key PROJ, got %s", key)
	}
}

func TestGetFirstProjectKey_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("[]")) // No projects
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	_, err := client.GetFirstProjectKey(context.Background())
	if err == nil {
		t.Fatal("Expected error for no projects")
	}
}
