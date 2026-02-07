package jira

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClient_ExtraMethods(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	client := NewClient(server.URL, "user", "token")

	// DeleteIssue
	mux.HandleFunc("/rest/api/3/issue/DEL-1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			w.WriteHeader(http.StatusNoContent)
		} else {
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	if err := client.DeleteIssue(context.Background(), "DEL-1"); err != nil {
		t.Errorf("DeleteIssue failed: %v", err)
	}

	// CreateTicket
	mux.HandleFunc("/rest/api/3/issue", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			var payload map[string]interface{}
			json.NewDecoder(r.Body).Decode(&payload)
			fields := payload["fields"].(map[string]interface{})
			// Standard CreateTicket doesn't verify parent
			if fields["summary"] == "New Ticket" {
				w.WriteHeader(http.StatusCreated)
				json.NewEncoder(w).Encode(map[string]string{"key": "NEW-1"})
			} else if fields["summary"] == "Child Ticket" {
				// For CreateChildTicket test below if reused, but we use separate handler logic usually
				// But since we are reusing mux for CreateChildTicket in separate test func, we should be careful.
				// Oh, I am putting CreateChildTicket in a separate function.
				w.WriteHeader(http.StatusCreated)
				json.NewEncoder(w).Encode(map[string]string{"key": "CHILD-1"})
			} else {
				w.WriteHeader(http.StatusBadRequest)
			}
		} else {
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	key, err := client.CreateTicket(context.Background(), "PROJ", "New Ticket", "Desc", "Task", []string{"label"})
	if err != nil {
		t.Errorf("CreateTicket failed: %v", err)
	}
	if key != "NEW-1" {
		t.Errorf("Expected key NEW-1, got %s", key)
	}

	// SearchIssues
	mux.HandleFunc("/rest/api/3/search/jql", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			q := r.URL.Query().Get("jql")
			if q == "project = PROJ" {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"issues": []interface{}{
						map[string]interface{}{"key": "PROJ-1"},
					},
				})
			} else {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{"issues": []interface{}{}})
			}
		}
	})

	issues, err := client.SearchIssues(context.Background(), "project = PROJ")
	if err != nil {
		t.Errorf("SearchIssues failed: %v", err)
	}
	if len(issues) != 1 {
		t.Errorf("Expected 1 issue, got %d", len(issues))
	}

	// AddIssueLink
	mux.HandleFunc("/rest/api/3/issueLink", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			w.WriteHeader(http.StatusCreated)
		}
	})

	if err := client.AddIssueLink(context.Background(), "A", "B", "Blocks"); err != nil {
		t.Errorf("AddIssueLink failed: %v", err)
	}

	// SetParent
	mux.HandleFunc("/rest/api/3/issue/CHILD-1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PUT" {
			var payload map[string]interface{}
			json.NewDecoder(r.Body).Decode(&payload)
			fields := payload["fields"].(map[string]interface{})
			parent := fields["parent"].(map[string]interface{})
			if parent["key"] == "PARENT-1" {
				w.WriteHeader(http.StatusNoContent)
			} else {
				w.WriteHeader(http.StatusBadRequest)
			}
		}
	})

	if err := client.SetParent(context.Background(), "CHILD-1", "PARENT-1"); err != nil {
		t.Errorf("SetParent failed: %v", err)
	}

	// AddLabel
	mux.HandleFunc("/rest/api/3/issue/LABEL-1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PUT" {
			w.WriteHeader(http.StatusNoContent)
		}
	})

	if err := client.AddLabel(context.Background(), "LABEL-1", "new-label"); err != nil {
		t.Errorf("AddLabel failed: %v", err)
	}

	// GetFirstProjectKey
	mux.HandleFunc("/rest/api/3/project", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{"key": "PROJ1"},
			})
		}
	})

	pkey, err := client.GetFirstProjectKey(context.Background())
	if err != nil {
		t.Errorf("GetFirstProjectKey failed: %v", err)
	}
	if pkey != "PROJ1" {
		t.Errorf("Expected PROJ1, got %s", pkey)
	}
}

func TestClient_GetBlockers(t *testing.T) {
	client := &Client{}

	ticket := map[string]interface{}{
		"fields": map[string]interface{}{
			"issuelinks": []interface{}{
				map[string]interface{}{
					"type": map[string]interface{}{"inward": "is blocked by"},
					"inwardIssue": map[string]interface{}{
						"key": "BLOCK-1",
						"fields": map[string]interface{}{
							"status": map[string]interface{}{"name": "In Progress"},
						},
					},
				},
				map[string]interface{}{
					"type": map[string]interface{}{"inward": "relates to"},
				},
				map[string]interface{}{
					"type": map[string]interface{}{"inward": "is blocked by"},
					"inwardIssue": map[string]interface{}{
						"key": "DONE-1",
						"fields": map[string]interface{}{
							"status": map[string]interface{}{"name": "Done"},
						},
					},
				},
			},
		},
	}

	keys := client.GetBlockerKeys(ticket)
	if len(keys) != 1 || keys[0] != "BLOCK-1" {
		t.Errorf("Expected [BLOCK-1], got %v", keys)
	}

	blockers := client.GetBlockers(ticket)
	if len(blockers) != 1 || blockers[0] != "BLOCK-1 (In Progress)" {
		t.Errorf("Expected [BLOCK-1 (In Progress)], got %v", blockers)
	}
}

func TestClient_Transitions(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	client := NewClient(server.URL, "user", "token")

	// GetTransitions
	mux.HandleFunc("/rest/api/3/issue/TRANS-1/transitions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"transitions": []interface{}{
					map[string]interface{}{"id": "11", "name": "Start"},
				},
			})
		} else if r.Method == "POST" {
			// Used by SmartTransition -> TransitionIssue
			w.WriteHeader(http.StatusNoContent)
		}
	})

	trans, err := client.GetTransitions(context.Background(), "TRANS-1")
	if err != nil {
		t.Errorf("GetTransitions failed: %v", err)
	}
	if len(trans) != 1 {
		t.Errorf("Expected 1 transition, got %d", len(trans))
	}

	// SmartTransition
	if err := client.SmartTransition(context.Background(), "TRANS-1", "Start"); err != nil {
		t.Errorf("SmartTransition failed: %v", err)
	}

	if err := client.SmartTransition(context.Background(), "TRANS-1", "Missing"); err == nil {
		t.Error("Expected error for missing transition")
	}
}

func TestClient_LoadLabelIssues(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	client := NewClient(server.URL, "user", "token")

	mux.HandleFunc("/rest/api/3/search/jql", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("jql")
		if strings.Contains(q, "labels = \"test-label\"") {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"issues": []interface{}{
					map[string]interface{}{"key": "LBL-1"},
				},
			})
		} else {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"issues": []interface{}{}})
		}
	})

	issues, err := client.LoadLabelIssues(context.Background(), "test-label")
	if err != nil {
		t.Errorf("LoadLabelIssues failed: %v", err)
	}
	if len(issues) != 1 {
		t.Errorf("Expected 1 issue, got %d", len(issues))
	}
}

func TestClient_CreateChildTicket(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	client := NewClient(server.URL, "user", "token")

	mux.HandleFunc("/rest/api/3/issue", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			var payload map[string]interface{}
			json.NewDecoder(r.Body).Decode(&payload)
			fields := payload["fields"].(map[string]interface{})
			parent, ok := fields["parent"].(map[string]interface{})
			if ok && parent["key"] == "PARENT-1" {
				w.WriteHeader(http.StatusCreated)
				json.NewEncoder(w).Encode(map[string]string{"key": "CHILD-1"})
			} else {
				w.WriteHeader(http.StatusBadRequest)
			}
		}
	})

	key, err := client.CreateChildTicket(context.Background(), "PROJ", "Sum", "Desc", "Sub-task", "PARENT-1", nil)
	if err != nil {
		t.Errorf("CreateChildTicket failed: %v", err)
	}
	if key != "CHILD-1" {
		t.Errorf("Expected key CHILD-1, got %s", key)
	}
}
