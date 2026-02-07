package jira

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAddIssueLink_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issueLink" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var payload map[string]interface{}
		json.NewDecoder(r.Body).Decode(&payload)

		if payload["type"].(map[string]interface{})["name"] != "Blocks" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if payload["inwardIssue"].(map[string]interface{})["key"] != "IN-1" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if payload["outwardIssue"].(map[string]interface{})["key"] != "OUT-1" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	err := client.AddIssueLink(context.Background(), "IN-1", "OUT-1", "Blocks")
	if err != nil {
		t.Fatalf("AddIssueLink failed: %v", err)
	}
}

func TestSetParent_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue/CHILD-1" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method != "PUT" {
			w.WriteHeader(http.StatusMethodNotAllowed)
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
		if r.URL.Path != "/rest/api/3/issue/PROJ-1" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method != "PUT" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var payload map[string]interface{}
		json.NewDecoder(r.Body).Decode(&payload)

		update := payload["update"].(map[string]interface{})
		labels := update["labels"].([]interface{})
		add := labels[0].(map[string]interface{})
		if add["add"] != "mylabel" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	err := client.AddLabel(context.Background(), "PROJ-1", "mylabel")
	if err != nil {
		t.Fatalf("AddLabel failed: %v", err)
	}
}

func TestGetFirstProjectKey_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/project" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"key": "PROJ"}]`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	key, err := client.GetFirstProjectKey(context.Background())
	if err != nil {
		t.Fatalf("GetFirstProjectKey failed: %v", err)
	}
	if key != "PROJ" {
		t.Errorf("Expected PROJ, got %s", key)
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
			name: "No links",
			ticket: map[string]interface{}{
				"fields": map[string]interface{}{},
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
