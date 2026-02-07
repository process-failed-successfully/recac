package jira

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
					t.Errorf("expected blocker %q, got %q", tt.expected[i], b)
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

		var payload map[string]interface{}
		json.NewDecoder(r.Body).Decode(&payload)

		typeName := payload["type"].(map[string]interface{})["name"]
		inward := payload["inwardIssue"].(map[string]interface{})["key"]
		outward := payload["outwardIssue"].(map[string]interface{})["key"]

		if typeName != "Blocks" || inward != "A-1" || outward != "B-1" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	err := client.AddIssueLink(context.Background(), "A-1", "B-1", "Blocks")
	if err != nil {
		t.Fatalf("AddIssueLink failed: %v", err)
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
	err := client.SetParent(context.Background(), "CHILD-1", "PARENT-1")
	if err != nil {
		t.Fatalf("SetParent failed: %v", err)
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

		if add != "urgent" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	err := client.AddLabel(context.Background(), "PROJ-1", "urgent")
	if err != nil {
		t.Fatalf("AddLabel failed: %v", err)
	}
}

func TestGetFirstProjectKey_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/project" || r.Method != "GET" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("[{\"key\": \"PROJ\"}, {\"key\": \"OTHER\"}]"))
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

func TestGetFirstProjectKey_NoProjects(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("[]"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	_, err := client.GetFirstProjectKey(context.Background())
	if err == nil {
		t.Fatal("Expected error for no projects")
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
		t.Fatal("Expected error for server failure")
	}
}
