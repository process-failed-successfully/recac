package jira

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// --- Mocks ---

type mockRoundTripper struct {
	response *http.Response
	err      error
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.response, m.err
}

// --- Helpers ---

func newTestClient(handler http.Handler) (*Client, *httptest.Server) {
	server := httptest.NewServer(handler)
	client := NewClient(server.URL, "user", "token")
	return client, server
}

// --- Tests ---

func TestClient_Authenticate(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		client, server := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		err := client.Authenticate(context.Background())
		if err != nil {
			t.Errorf("Authenticate() returned an unexpected error: %v", err)
		}
	})

	t.Run("failure", func(t *testing.T) {
		client, server := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer server.Close()

		err := client.Authenticate(context.Background())
		if err == nil {
			t.Error("Authenticate() expected an error but got none")
		}
	})
}

func TestClient_GetTicket(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		client, server := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"key": "TEST-1"})
		}))
		defer server.Close()

		ticket, err := client.GetTicket(context.Background(), "TEST-1")
		if err != nil {
			t.Fatalf("GetTicket() returned an unexpected error: %v", err)
		}
		if key, _ := ticket["key"].(string); key != "TEST-1" {
			t.Errorf("expected ticket key 'TEST-1', got '%s'", key)
		}
	})

	t.Run("not found", func(t *testing.T) {
		client, server := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		_, err := client.GetTicket(context.Background(), "TEST-1")
		if err == nil {
			t.Error("GetTicket() expected an error but got none")
		}
	})
}

func TestClient_CreateTicket(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		client, server := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]string{"key": "TEST-1"})
		}))
		defer server.Close()

		key, err := client.CreateTicket(context.Background(), "PROJ", "summary", "desc", "Story", nil)
		if err != nil {
			t.Fatalf("CreateTicket() returned an unexpected error: %v", err)
		}
		if key != "TEST-1" {
			t.Errorf("expected new ticket key 'TEST-1', got '%s'", key)
		}
	})

	t.Run("failure", func(t *testing.T) {
		client, server := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer server.Close()

		_, err := client.CreateTicket(context.Background(), "PROJ", "summary", "desc", "Story", nil)
		if err == nil {
			t.Error("CreateTicket() expected an error but got none")
		}
	})
}

func TestClient_AddComment(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		client, server := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
		}))
		defer server.Close()

		err := client.AddComment(context.Background(), "TEST-1", "comment")
		if err != nil {
			t.Fatalf("AddComment() returned an unexpected error: %v", err)
		}
	})

	t.Run("failure", func(t *testing.T) {
		client, server := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer server.Close()

		err := client.AddComment(context.Background(), "TEST-1", "comment")
		if err == nil {
			t.Error("AddComment() expected an error but got none")
		}
	})
}

func TestClient_DeleteIssue(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		client, server := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		err := client.DeleteIssue(context.Background(), "TEST-1")
		if err != nil {
			t.Fatalf("DeleteIssue() returned an unexpected error: %v", err)
		}
	})

	t.Run("failure", func(t *testing.T) {
		client, server := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer server.Close()

		err := client.DeleteIssue(context.Background(), "TEST-1")
		if err == nil {
			t.Error("DeleteIssue() expected an error but got none")
		}
	})
}

func TestClient_AddIssueLink(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		client, server := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
		}))
		defer server.Close()

		err := client.AddIssueLink(context.Background(), "TEST-1", "TEST-2", "Blocks")
		if err != nil {
			t.Fatalf("AddIssueLink() returned an unexpected error: %v", err)
		}
	})

	t.Run("failure", func(t *testing.T) {
		client, server := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer server.Close()

		err := client.AddIssueLink(context.Background(), "TEST-1", "TEST-2", "Blocks")
		if err == nil {
			t.Error("AddIssueLink() expected an error but got none")
		}
	})
}

func TestClient_SetParent(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		client, server := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		err := client.SetParent(context.Background(), "TEST-1", "TEST-2")
		if err != nil {
			t.Fatalf("SetParent() returned an unexpected error: %v", err)
		}
	})

	t.Run("failure", func(t *testing.T) {
		client, server := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer server.Close()

		err := client.SetParent(context.Background(), "TEST-1", "TEST-2")
		if err == nil {
			t.Error("SetParent() expected an error but got none")
		}
	})
}

func TestClient_AddLabel(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		client, server := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		err := client.AddLabel(context.Background(), "TEST-1", "label")
		if err != nil {
			t.Fatalf("AddLabel() returned an unexpected error: %v", err)
		}
	})

	t.Run("failure", func(t *testing.T) {
		client, server := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer server.Close()

		err := client.AddLabel(context.Background(), "TEST-1", "label")
		if err == nil {
			t.Error("AddLabel() expected an error but got none")
		}
	})
}

func TestClient_SmartTransition(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		client, server := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/rest/api/3/issue/TEST-1/transitions" && r.Method == "GET" {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"transitions": []map[string]interface{}{
						{"id": "1", "name": "To Do"},
						{"id": "2", "name": "In Progress"},
					},
				})
			} else if r.URL.Path == "/rest/api/3/issue/TEST-1/transitions" && r.Method == "POST" {
				w.WriteHeader(http.StatusNoContent)
			}
		}))
		defer server.Close()

		err := client.SmartTransition(context.Background(), "TEST-1", "In Progress")
		if err != nil {
			t.Fatalf("SmartTransition() returned an unexpected error: %v", err)
		}
	})

	t.Run("failure", func(t *testing.T) {
		client, server := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer server.Close()

		err := client.SmartTransition(context.Background(), "TEST-1", "In Progress")
		if err == nil {
			t.Error("SmartTransition() expected an error but got none")
		}
	})
}

func TestClient_GetFirstProjectKey(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		client, server := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{"key": "PROJ"},
			})
		}))
		defer server.Close()
		key, err := client.GetFirstProjectKey(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if key != "PROJ" {
			t.Errorf("expected key 'PROJ', got '%s'", key)
		}
	})
	t.Run("no projects", func(t *testing.T) {
		client, server := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]map[string]interface{}{})
		}))
		defer server.Close()
		_, err := client.GetFirstProjectKey(context.Background())
		if err == nil {
			t.Fatal("expected an error but got none")
		}
	})
	t.Run("invalid format", func(t *testing.T) {
		client, server := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{"id": "123"},
			})
		}))
		defer server.Close()
		_, err := client.GetFirstProjectKey(context.Background())
		if err == nil {
			t.Fatal("expected an error but got none")
		}
	})
}
