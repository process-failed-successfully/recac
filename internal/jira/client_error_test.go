package jira

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuthenticate_NetworkError(t *testing.T) {
	client := NewClient("http://invalid-url", "user", "token")
	client.HTTPClient = &http.Client{
		Transport: &errorTransport{},
	}

	err := client.Authenticate(context.Background())
	if err == nil {
		t.Error("Expected error for network failure")
	}
}

func TestGetTicket_DecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid-json"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	_, err := client.GetTicket(context.Background(), "T-1")
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "failed to decode response") {
		t.Errorf("Expected decode error, got %v", err)
	}
}

func TestAddComment_ErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	err := client.AddComment(context.Background(), "T-1", "comment")
	if err == nil {
		t.Error("Expected error for 500 status")
	}
}

func TestSearchIssues_ErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	_, err := client.SearchIssues(context.Background(), "jql")
	if err == nil {
		t.Error("Expected error for bad request")
	}
}

func TestGetTransitions_DecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{")) // incomplete json
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	_, err := client.GetTransitions(context.Background(), "T-1")
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestCreateTicket_DecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("not-json"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	_, err := client.CreateTicket(context.Background(), "P", "S", "D", "T", nil)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestGetFirstProjectKey_DecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not-json"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	_, err := client.GetFirstProjectKey(context.Background())
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

type errorTransport struct{}

func (t *errorTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, io.ErrUnexpectedEOF
}
