package agent

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenRouterClient_HTTP_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Header.Get("X-Title") != "Process Failed Successfully" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"choices": [{"message": {"content": "Hello form OpenRouter"}}]}`)
	}))
	defer server.Close()

	client := NewOpenRouterClient("test-key", "model", "test-project")
	client.apiURL = server.URL

	resp, err := client.Send(context.Background(), "Hi")
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if resp != "Hello form OpenRouter" {
		t.Errorf("Expected 'Hello form OpenRouter', got %q", resp)
	}
}

func TestOpenRouterClient_HTTP_Errors(t *testing.T) {
	tests := []struct {
		name       string
		handler    func(w http.ResponseWriter, r *http.Request)
		wantErr    bool
		errMessage string
	}{
		{
			name: "Status 500",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprint(w, "Internal Server Error")
			},
			wantErr:    true,
			errMessage: "OpenRouter API returned status 500",
		},
		{
			name: "Malformed JSON",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				fmt.Fprint(w, "{invalid-json")
			},
			wantErr:    true,
			errMessage: "failed to decode response",
		},
		{
			name: "Empty Choices",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				fmt.Fprint(w, `{"choices": []}`)
			},
			wantErr:    true,
			errMessage: "no content in response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.handler))
			defer server.Close()

			client := NewOpenRouterClient("test-key", "model", "test-project")
			client.apiURL = server.URL

			_, err := client.Send(context.Background(), "Hi")
			if (err != nil) != tt.wantErr {
				t.Errorf("Send() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenRouterClient_SendStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// SSE format
		fmt.Fprint(w, "data: {\"choices\": [{\"delta\": {\"content\": \"Hello \"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\": [{\"delta\": {\"content\": \"World\"}}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	client := NewOpenRouterClient("test-key", "model", "test-project")
	client.apiURL = server.URL

	var fullChunk string
	resp, err := client.SendStream(context.Background(), "Hi", func(c string) {
		fullChunk += c
	})
	if err != nil {
		t.Fatalf("SendStream failed: %v", err)
	}
	if resp != "Hello World" {
		t.Errorf("Expected 'Hello World', got %q", resp)
	}
	if fullChunk != "Hello World" {
		t.Errorf("Expected chunk 'Hello World', got %q", fullChunk)
	}
}
