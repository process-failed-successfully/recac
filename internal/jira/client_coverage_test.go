package jira

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDeleteIssue_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	err := client.DeleteIssue(context.Background(), "INVALID")
	if err == nil {
		t.Error("Expected error for INVALID issue")
	}
}

func TestAddIssueLink(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issueLink" {
			w.WriteHeader(http.StatusNotFound)
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

func TestSetParent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PUT" && strings.Contains(r.URL.Path, "/issue/SUB-1") {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	err := client.SetParent(context.Background(), "SUB-1", "PARENT-1")
	if err != nil {
		t.Fatalf("SetParent failed: %v", err)
	}
}

func TestAddLabel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PUT" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	err := client.AddLabel(context.Background(), "PROJ-1", "new-label")
	if err != nil {
		t.Fatalf("AddLabel failed: %v", err)
	}
}

func TestGetFirstProjectKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"key": "FIRST"}, {"key": "SECOND"}]`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	key, err := client.GetFirstProjectKey(context.Background())
	if err != nil {
		t.Fatalf("GetFirstProjectKey failed: %v", err)
	}
	if key != "FIRST" {
		t.Errorf("Expected FIRST, got %s", key)
	}
}

func TestGetFirstProjectKey_Empty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	_, err := client.GetFirstProjectKey(context.Background())
	if err == nil {
		t.Error("Expected error for empty project list")
	}
}

func TestTransitionIssue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && strings.Contains(r.URL.Path, "/transitions") {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	err := client.TransitionIssue(context.Background(), "TKT-1", "21")
	if err != nil {
		t.Fatalf("TransitionIssue failed: %v", err)
	}
}

func TestTransitionIssue_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	err := client.TransitionIssue(context.Background(), "TKT-1", "21")
	if err == nil {
		t.Fatalf("Expected TransitionIssue to fail")
	}
}

func TestAddIssueLink_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	err := client.AddIssueLink(context.Background(), "A-1", "B-1", "Blocks")
	if err == nil {
		t.Fatalf("Expected AddIssueLink to fail")
	}
}

func TestSetParent_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	err := client.SetParent(context.Background(), "SUB-1", "PARENT-1")
	if err == nil {
		t.Fatalf("Expected SetParent to fail")
	}
}

func TestAddLabel_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	err := client.AddLabel(context.Background(), "PROJ-1", "new-label")
	if err == nil {
		t.Fatalf("Expected AddLabel to fail")
	}
}

func TestGetTransitions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && strings.Contains(r.URL.Path, "/transitions") {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"transitions": [{"id": "11", "name": "In Progress"}]}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	transitions, err := client.GetTransitions(context.Background(), "TKT-1")
	if err != nil {
		t.Fatalf("GetTransitions failed: %v", err)
	}
	if len(transitions) != 1 {
		t.Errorf("Expected 1 transition, got %d", len(transitions))
	}
}

func TestGetTransitions_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	_, err := client.GetTransitions(context.Background(), "TKT-1")
	if err == nil {
		t.Fatalf("Expected GetTransitions to fail")
	}
}

func TestGetFirstProjectKey_ErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	_, err := client.GetFirstProjectKey(context.Background())
	if err == nil {
		t.Fatalf("Expected GetFirstProjectKey to fail")
	}
}

func TestGetFirstProjectKey_BadFormat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"key": 123}]`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	_, err := client.GetFirstProjectKey(context.Background())
	if err == nil {
		t.Fatalf("Expected GetFirstProjectKey to fail with wrong type")
	}
}

func TestAuthenticate_ErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	err := client.Authenticate(context.Background())
	if err == nil {
		t.Fatalf("Expected Authenticate to fail")
	}
}

func TestSearchIssues_ErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	_, err := client.SearchIssues(context.Background(), "project=ABC")
	if err == nil {
		t.Fatalf("Expected SearchIssues to fail")
	}
}

func TestGetTicket_ErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	_, err := client.GetTicket(context.Background(), "TKT-1")
	if err == nil {
		t.Fatalf("Expected GetTicket to fail")
	}
}

func TestCreateTicket_ErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	_, err := client.CreateTicket(context.Background(), "PROJ", "Summary", "Desc", "Task", []string{})
	if err == nil {
		t.Fatalf("Expected CreateTicket to fail")
	}
}

func TestAddComment_ErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	err := client.AddComment(context.Background(), "TKT-1", "Comment")
	if err == nil {
		t.Fatalf("Expected AddComment to fail")
	}
}
