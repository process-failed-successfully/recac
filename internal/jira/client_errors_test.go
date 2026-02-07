package jira

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// MockRoundTripper simulates network errors
type MockRoundTripper struct {
	Err error
}

func (m *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, m.Err
}

func TestClient_NetworkErrors(t *testing.T) {
	client := NewClient("http://jira.local", "user", "token")
	client.HTTPClient.Transport = &MockRoundTripper{Err: errors.New("network error")}

	ctx := context.Background()

	t.Run("Authenticate", func(t *testing.T) {
		if err := client.Authenticate(ctx); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("GetTicket", func(t *testing.T) {
		if _, err := client.GetTicket(ctx, "PROJ-1"); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("TransitionIssue", func(t *testing.T) {
		if err := client.TransitionIssue(ctx, "PROJ-1", "31"); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("AddComment", func(t *testing.T) {
		if err := client.AddComment(ctx, "PROJ-1", "comment"); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("DeleteIssue", func(t *testing.T) {
		if err := client.DeleteIssue(ctx, "PROJ-1"); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("CreateTicket", func(t *testing.T) {
		if _, err := client.CreateTicket(ctx, "PROJ", "Summary", "Desc", "Task", nil); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("SearchIssues", func(t *testing.T) {
		if _, err := client.SearchIssues(ctx, "jql"); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("GetTransitions", func(t *testing.T) {
		if _, err := client.GetTransitions(ctx, "PROJ-1"); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("AddIssueLink", func(t *testing.T) {
		if err := client.AddIssueLink(ctx, "IN-1", "OUT-1", "Blocks"); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("SetParent", func(t *testing.T) {
		if err := client.SetParent(ctx, "SUB-1", "PAR-1"); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("AddLabel", func(t *testing.T) {
		if err := client.AddLabel(ctx, "PROJ-1", "label"); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("GetFirstProjectKey", func(t *testing.T) {
		if _, err := client.GetFirstProjectKey(ctx); err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestClient_StatusErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	ctx := context.Background()

	t.Run("Authenticate", func(t *testing.T) {
		if err := client.Authenticate(ctx); err == nil {
			t.Error("expected error, got nil")
		} else if !strings.Contains(err.Error(), "500") {
			t.Errorf("expected 500 error, got %v", err)
		}
	})

	t.Run("GetTicket", func(t *testing.T) {
		if _, err := client.GetTicket(ctx, "PROJ-1"); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("TransitionIssue", func(t *testing.T) {
		if err := client.TransitionIssue(ctx, "PROJ-1", "31"); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("AddComment", func(t *testing.T) {
		if err := client.AddComment(ctx, "PROJ-1", "comment"); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("DeleteIssue", func(t *testing.T) {
		if err := client.DeleteIssue(ctx, "PROJ-1"); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("CreateTicket", func(t *testing.T) {
		if _, err := client.CreateTicket(ctx, "PROJ", "Summary", "Desc", "Task", nil); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("SearchIssues", func(t *testing.T) {
		if _, err := client.SearchIssues(ctx, "jql"); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("GetTransitions", func(t *testing.T) {
		if _, err := client.GetTransitions(ctx, "PROJ-1"); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("AddIssueLink", func(t *testing.T) {
		if err := client.AddIssueLink(ctx, "IN-1", "OUT-1", "Blocks"); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("SetParent", func(t *testing.T) {
		if err := client.SetParent(ctx, "SUB-1", "PAR-1"); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("AddLabel", func(t *testing.T) {
		if err := client.AddLabel(ctx, "PROJ-1", "label"); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("GetFirstProjectKey", func(t *testing.T) {
		if _, err := client.GetFirstProjectKey(ctx); err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestClient_JSONErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	ctx := context.Background()

	t.Run("GetTicket", func(t *testing.T) {
		if _, err := client.GetTicket(ctx, "PROJ-1"); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("CreateTicket", func(t *testing.T) {
		// CreateTicket expects 201 Created usually, but 200 OK is also checked.
		// If 200 OK but invalid JSON, it should fail decoding.
		if _, err := client.CreateTicket(ctx, "PROJ", "Summary", "Desc", "Task", nil); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("SearchIssues", func(t *testing.T) {
		if _, err := client.SearchIssues(ctx, "jql"); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("GetTransitions", func(t *testing.T) {
		if _, err := client.GetTransitions(ctx, "PROJ-1"); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("GetFirstProjectKey", func(t *testing.T) {
		if _, err := client.GetFirstProjectKey(ctx); err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestClient_SmartTransition_Errors(t *testing.T) {
	// Case 1: Error fetching transitions (Network)
	client := NewClient("http://jira.local", "user", "token")
	client.HTTPClient.Transport = &MockRoundTripper{Err: errors.New("network error")}
	ctx := context.Background()

	if err := client.SmartTransition(ctx, "PROJ-1", "In Progress"); err == nil {
		t.Error("expected error fetching transitions, got nil")
	}

	// Case 2: No transition found
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/transitions") {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"transitions": [{"id": "10", "name": "Closed"}]}`))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client = NewClient(server.URL, "user", "token")
	if err := client.SmartTransition(ctx, "PROJ-1", "In Progress"); err == nil {
		t.Error("expected error for missing transition, got nil")
	} else if !strings.Contains(err.Error(), "no transition found") {
		t.Errorf("expected 'no transition found' error, got %v", err)
	}
}

func TestClient_GetFirstProjectKey_Empty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	ctx := context.Background()

	if _, err := client.GetFirstProjectKey(ctx); err == nil {
		t.Error("expected error for empty project list, got nil")
	}
}

func TestClient_GetBlockers(t *testing.T) {
	client := NewClient("http://jira.local", "user", "token")

	// Case 1: Invalid fields
	ticket := map[string]interface{}{}
	if res := client.GetBlockers(ticket); res != nil {
		t.Errorf("expected nil for invalid ticket, got %v", res)
	}

	// Case 2: Valid blocker
	ticket = map[string]interface{}{
		"fields": map[string]interface{}{
			"issuelinks": []interface{}{
				map[string]interface{}{
					"type": map[string]interface{}{
						"inward": "is blocked by",
					},
					"inwardIssue": map[string]interface{}{
						"key": "BLK-1",
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
		t.Errorf("expected 1 blocker, got %d", len(blockers))
	}
	if !strings.Contains(blockers[0], "BLK-1") {
		t.Errorf("expected blocker BLK-1, got %v", blockers[0])
	}

	// Case 3: Done blocker (should be ignored)
	ticket = map[string]interface{}{
		"fields": map[string]interface{}{
			"issuelinks": []interface{}{
				map[string]interface{}{
					"type": map[string]interface{}{
						"inward": "is blocked by",
					},
					"inwardIssue": map[string]interface{}{
						"key": "BLK-2",
						"fields": map[string]interface{}{
							"status": map[string]interface{}{
								"name": "Done",
							},
						},
					},
				},
			},
		},
	}

	blockers = client.GetBlockers(ticket)
	if len(blockers) != 0 {
		t.Errorf("expected 0 blockers, got %d", len(blockers))
	}
}
