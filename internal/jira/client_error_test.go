package jira

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateTicket_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"errorMessages":["Field 'summary' is required"]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	_, err := client.CreateTicket(context.Background(), "PROJ", "", "Desc", "Task", nil)
	if err == nil {
		t.Error("Expected error for bad request")
	}
}

func TestTransitionIssue_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"errorMessages":["Transition not valid"]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	err := client.TransitionIssue(context.Background(), "PROJ-1", "99")
	if err == nil {
		t.Error("Expected error for invalid transition")
	}
}

func TestAddIssueLink_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"errorMessages":["Issue not found"]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	err := client.AddIssueLink(context.Background(), "PROJ-1", "PROJ-2", "Blocks")
	if err == nil {
		t.Error("Expected error for non-existent issue")
	}
}

func TestSetParent_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"errorMessages":["Permission denied"]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	err := client.SetParent(context.Background(), "PROJ-2", "PROJ-1")
	if err == nil {
		t.Error("Expected error for permission denied")
	}
}

func TestAddLabel_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	err := client.AddLabel(context.Background(), "PROJ-1", "label")
	if err == nil {
		t.Error("Expected error for server error")
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
		t.Error("Expected error for non-existent issue")
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
