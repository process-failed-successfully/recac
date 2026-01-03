package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGeminiClient_HTTP_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify URL format
		if r.URL.Path != "/gemini-pro:generateContent" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get("x-goog-api-key") != "test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Verify body
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		// Basic check
		if body["contents"] == nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"candidates": [
				{
					"content": {
						"parts": [
							{
								"text": "Hello form Gemini"
							}
						]
					}
				}
			]
		}`))
	}))
	defer server.Close()

	client := NewGeminiClient("test-key", "gemini-pro", "test-project")
	// Inject fast backoff to prevent slow retries on transient failures
	client.backoffFn = func(i int) time.Duration { return time.Millisecond }
	// Override API URL to point to mock server
	client.apiURL = server.URL

	resp, err := client.Send(context.Background(), "Hi")
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if resp != "Hello form Gemini" {
		t.Errorf("Expected 'Hello form Gemini', got %q", resp)
	}
}

func TestGeminiClient_HTTP_Errors(t *testing.T) {
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
				w.Write([]byte("Internal Server Error"))
			},
			wantErr:    true,
			errMessage: "API returned status 500",
		},
		{
			name: "Malformed JSON",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("{invalid-json"))
			},
			wantErr:    true,
			errMessage: "failed to decode response",
		},
		{
			name: "Empty Candidates",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"candidates": []}`))
			},
			wantErr:    true,
			errMessage: "no content in response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.handler))
			defer server.Close()

			client := NewGeminiClient("test-key", "gemini-pro", "test-project")
			client.backoffFn = func(i int) time.Duration { return time.Millisecond }
			client.apiURL = server.URL

			_, err := client.Send(context.Background(), "Hi")
			if (err != nil) != tt.wantErr {
				t.Errorf("Send() error = %v, wantErr %v", err, tt.wantErr)
			}
			// Note: Send retries, so error might be wrapped
			// We mainly check it fails eventually
		})
	}
}

func TestGeminiClient_SendStream(t *testing.T) {
	// SendStream just calls Send for now
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"candidates": [
				{
					"content": {
						"parts": [
							{
								"text": "Streamed Response"
							}
						]
					}
				}
			]
		}`))
	}))
	defer server.Close()

	client := NewGeminiClient("test-key", "gemini-pro", "test-project")
	client.backoffFn = func(i int) time.Duration { return time.Millisecond }
	client.apiURL = server.URL

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
