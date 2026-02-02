package agent

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOllamaClient_ListModels(t *testing.T) {
	// Mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Errorf("Expected path /api/tags, got %s", r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		fmt.Fprintln(w, `{"models": [{"name": "llama3"}, {"name": "mistral"}]}`)
	}))
	defer server.Close()

	// Initialize client with mock server URL
	client := NewOllamaClient(server.URL, "llama3", "test")

	models, err := client.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}

	if len(models) != 2 {
		t.Errorf("Expected 2 models, got %d", len(models))
	}
	if models[0] != "llama3" || models[1] != "mistral" {
		t.Errorf("Unexpected models: %v", models)
	}
}

// Custom RoundTripper to intercept requests
type mockTransport struct {
	handler func(*http.Request) (*http.Response, error)
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.handler(req)
}

func TestOpenAIClient_ListModels(t *testing.T) {
	client := NewOpenAIClient("test-key", "gpt-4", "test")

	// Replace httpClient with one using our mock transport
	mock := &mockTransport{
		handler: func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != "https://api.openai.com/v1/models" {
				return nil, fmt.Errorf("Unexpected URL: %s", req.URL.String())
			}

			respBody := `{"data": [{"id": "gpt-4"}, {"id": "gpt-3.5-turbo"}]}`

			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(respBody)),
				Header:     make(http.Header),
			}, nil
		},
	}
	client.httpClient = &http.Client{Transport: mock}

	models, err := client.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}

	if len(models) != 2 {
		t.Errorf("Expected 2 models, got %d", len(models))
	}
	if models[0] != "gpt-4" || models[1] != "gpt-3.5-turbo" {
		t.Errorf("Unexpected models: %v", models)
	}
}
