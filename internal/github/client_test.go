package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_Authenticate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user" {
			t.Errorf("Expected path /user, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "token test-token" {
			t.Errorf("Expected Authorization header, got %s", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"login": "testuser"}`))
	}))
	defer server.Close()

	client := NewClient("test-token", "owner", "repo")
	client.BaseURL = server.URL

	err := client.Authenticate(context.Background())
	if err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}
}

func TestClient_CreateTicket(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/repos/owner/repo/issues" {
			t.Errorf("Expected path /repos/owner/repo/issues, got %s", r.URL.Path)
		}

		var payload map[string]interface{}
		json.NewDecoder(r.Body).Decode(&payload)

		if payload["title"] != "Summary" {
			t.Errorf("Expected title Summary, got %v", payload["title"])
		}
		if payload["body"] != "Description" {
			t.Errorf("Expected body Description, got %v", payload["body"])
		}
		labels := payload["labels"].([]interface{})
		found := false
		for _, l := range labels {
			if l == "kind/story" {
				found = true
			}
		}
		if !found {
			t.Errorf("Expected kind/story label, got %v", labels)
		}

		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"number": 42, "html_url": "http://github.com/owner/repo/issues/42"}`))
	}))
	defer server.Close()

	client := NewClient("test-token", "owner", "repo")
	client.BaseURL = server.URL

	id, err := client.CreateTicket(context.Background(), "", "Summary", "Description", "Story", []string{"foo"})
	if err != nil {
		t.Fatalf("CreateTicket failed: %v", err)
	}
	if id != "GH-42" {
		t.Errorf("Expected ID GH-42, got %s", id)
	}
}

func TestClient_CreateChildTicket(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		json.NewDecoder(r.Body).Decode(&payload)

		body := payload["body"].(string)
		if body != "Child\n\nParent: #42" {
			t.Errorf("Expected body with parent link, got %s", body)
		}

		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"number": 43}`))
	}))
	defer server.Close()

	client := NewClient("test-token", "owner", "repo")
	client.BaseURL = server.URL

	_, err := client.CreateChildTicket(context.Background(), "", "Child", "Child", "Task", "GH-42", nil)
	if err != nil {
		t.Fatalf("CreateChildTicket failed: %v", err)
	}
}

func TestClient_AddIssueLink(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo/issues/43/comments" {
			t.Errorf("Expected path /repos/owner/repo/issues/43/comments, got %s", r.URL.Path)
		}
		var payload map[string]interface{}
		json.NewDecoder(r.Body).Decode(&payload)
		if payload["body"] != "Blocked by #42" {
			t.Errorf("Expected comment Blocked by #42, got %v", payload["body"])
		}

		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := NewClient("test-token", "owner", "repo")
	client.BaseURL = server.URL

	err := client.AddIssueLink(context.Background(), "GH-42", "GH-43", "Blocks")
	if err != nil {
		t.Fatalf("AddIssueLink failed: %v", err)
	}
}
