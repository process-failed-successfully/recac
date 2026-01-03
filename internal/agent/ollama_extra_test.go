package agent

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOllamaClient_HTTP_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"response": "Hello form Ollama", "done": true}`)
	}))
	defer server.Close()

	client := NewOllamaClient(server.URL, "llama2", "test-project")

	resp, err := client.Send(context.Background(), "Hi")
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if resp != "Hello form Ollama" {
		t.Errorf("Expected 'Hello form Ollama', got %q", resp)
	}
}

func TestOllamaClient_HTTP_Errors(t *testing.T) {
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
			errMessage: "Ollama API returned status 500",
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
			name: "Ollama Error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				fmt.Fprint(w, `{"error": "model not found", "done": true}`)
			},
			wantErr:    true,
			errMessage: "Ollama API error: model not found",
		},
		{
			name: "Incomplete Response",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				fmt.Fprint(w, `{"response": "...", "done": false}`)
			},
			wantErr:    true,
			errMessage: "Ollama response incomplete",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.handler))
			defer server.Close()

			client := NewOllamaClient(server.URL, "llama2", "test-project")

			_, err := client.Send(context.Background(), "Hi")
			if (err != nil) != tt.wantErr {
				t.Errorf("Send() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOllamaClient_SendStream(t *testing.T) {
	// Ollama SendStream fallback calls Send
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"response": "Streamed Response", "done": true}`)
	}))
	defer server.Close()

	client := NewOllamaClient(server.URL, "llama2", "test-project")

	var chunk string
	resp, err := client.SendStream(context.Background(), "Hi", func(c string) {
		chunk = c
	})
	if err != nil {
		t.Fatalf("SendStream failed: %v", err)
	}
	if resp != "Streamed Response" {
		t.Errorf("Expected 'Streamed Response', got %q", resp)
	}
	if chunk != "Streamed Response" {
		t.Errorf("Expected chunk 'Streamed Response', got %q", chunk)
	}
}
