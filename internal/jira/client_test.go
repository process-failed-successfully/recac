package jira

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuthenticate_Success(t *testing.T) {
	// Mock Server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Auth Header
		user, pass, ok := r.BasicAuth()
		if !ok || user != "user" || pass != "token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if r.URL.Path != "/rest/api/3/myself" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"accountId": "123"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")

	if err := client.Authenticate(context.Background()); err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}
}

func TestGetTicket_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue/PROJ-123" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"key": "PROJ-123", "fields": {"summary": "Test Ticket"}}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	ticket, err := client.GetTicket(context.Background(), "PROJ-123")
	if err != nil {
		t.Fatalf("GetTicket failed: %v", err)
	}

	if ticket["key"] != "PROJ-123" {
		t.Errorf("Expected key PROJ-123, got %v", ticket["key"])
	}
}

func TestGetTicket_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue/NONEXIST-123" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"errorMessages":["Issue Does Not Exist"],"errors":{}}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	_, err := client.GetTicket(context.Background(), "NONEXIST-123")
	if err == nil {
		t.Fatal("Expected error for non-existent ticket")
	}

	if !strings.Contains(err.Error(), "404") && !strings.Contains(err.Error(), "failed to fetch ticket") {
		t.Errorf("Expected error message about failed fetch, got: %v", err)
	}
}

func TestTransitionIssue_Success(t *testing.T) {
	var receivedPath string
	var receivedMethod string
	var receivedTransitionID string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		receivedMethod = r.Method

		if r.URL.Path != "/rest/api/3/issue/PROJ-123/transitions" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		trans, ok := payload["transition"].(map[string]interface{})
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		receivedTransitionID, _ = trans["id"].(string)

		if receivedTransitionID != "31" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	err := client.TransitionIssue(context.Background(), "PROJ-123", "31")
	if err != nil {
		t.Fatalf("TransitionIssue failed: %v", err)
	}

	// Verify the mock backend received the transition request
	if receivedPath != "/rest/api/3/issue/PROJ-123/transitions" {
		t.Errorf("Expected path /rest/api/3/issue/PROJ-123/transitions, got %s", receivedPath)
	}
	if receivedMethod != "POST" {
		t.Errorf("Expected method POST, got %s", receivedMethod)
	}
	if receivedTransitionID != "31" {
		t.Errorf("Expected transition ID '31', got %q", receivedTransitionID)
	}
}

func TestAddComment_Success(t *testing.T) {
	var receivedComment string
	var receivedPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path

		if r.URL.Path != "/rest/api/3/issue/PROJ-123/comment" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Extract comment text from ADF format
		body, ok := payload["body"].(map[string]interface{})
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		content, ok := body["content"].([]interface{})
		if !ok || len(content) == 0 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		paragraph, ok := content[0].(map[string]interface{})
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		paragraphContent, ok := paragraph["content"].([]interface{})
		if !ok || len(paragraphContent) == 0 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		textNode, ok := paragraphContent[0].(map[string]interface{})
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		receivedComment, _ = textNode["text"].(string)

		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id": "12345", "body": {"type": "doc"}}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	err := client.AddComment(context.Background(), "PROJ-123", "This is a test comment")
	if err != nil {
		t.Fatalf("AddComment failed: %v", err)
	}

	if receivedPath != "/rest/api/3/issue/PROJ-123/comment" {
		t.Errorf("Expected path /rest/api/3/issue/PROJ-123/comment, got %s", receivedPath)
	}

	if receivedComment != "This is a test comment" {
		t.Errorf("Expected comment 'This is a test comment', got %q", receivedComment)
	}
}

func TestAddComment_FormattingPreserved(t *testing.T) {
	var receivedPayload map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedPayload)
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id": "12345"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	commentText := "Line 1\nLine 2\n*Bold* text"
	err := client.AddComment(context.Background(), "PROJ-456", commentText)
	if err != nil {
		t.Fatalf("AddComment failed: %v", err)
	}

	// Verify ADF structure is preserved
	body, ok := receivedPayload["body"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected 'body' field in payload")
	}

	if body["type"] != "doc" {
		t.Errorf("Expected body type 'doc', got %v", body["type"])
	}

	content, ok := body["content"].([]interface{})
	if !ok || len(content) == 0 {
		t.Fatal("Expected 'content' array in body")
	}

	paragraph, ok := content[0].(map[string]interface{})
	if !ok {
		t.Fatal("Expected paragraph in content")
	}

	if paragraph["type"] != "paragraph" {
		t.Errorf("Expected paragraph type 'paragraph', got %v", paragraph["type"])
	}
}

func TestAddComment_ErrorHandling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	err := client.AddComment(context.Background(), "INVALID-123", "test")
	if err == nil {
		t.Fatal("Expected error for non-existent ticket")
	}
}

func TestClient_ParseDescription(t *testing.T) {
	client := NewClient("http://jira.local", "user", "token")

	mockData := map[string]interface{}{
		"fields": map[string]interface{}{
			"description": map[string]interface{}{
				"version": 1,
				"type":    "doc",
				"content": []interface{}{
					map[string]interface{}{
						"type": "paragraph",
						"content": []interface{}{
							map[string]interface{}{
								"type": "text",
								"text": "Task: Implement Auth",
							},
						},
					},
					map[string]interface{}{
						"type": "paragraph",
						"content": []interface{}{
							map[string]interface{}{
								"type": "text",
								"text": "Details: Use OAuth2",
							},
						},
					},
				},
			},
		},
	}

	result := client.ParseDescription(mockData)
	expected := "Task: Implement Auth\nDetails: Use OAuth2\n"

	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}
