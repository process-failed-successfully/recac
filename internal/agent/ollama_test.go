package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewOllamaClient(t *testing.T) {
	// Test with default baseURL
	client := NewOllamaClient("", "llama2")
	if client.baseURL != "http://localhost:11434" {
		t.Errorf("expected default baseURL http://localhost:11434, got %s", client.baseURL)
	}
	if client.model != "llama2" {
		t.Errorf("expected model llama2, got %s", client.model)
	}

	// Test with custom baseURL
	client = NewOllamaClient("http://localhost:8080", "mistral")
	if client.baseURL != "http://localhost:8080" {
		t.Errorf("expected baseURL http://localhost:8080, got %s", client.baseURL)
	}
	if client.model != "mistral" {
		t.Errorf("expected model mistral, got %s", client.model)
	}
}

func TestOllamaClient_Send_Success(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/generate" {
			t.Errorf("expected /api/generate, got %s", r.URL.Path)
		}

		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		if reqBody["model"] != "llama2" {
			t.Errorf("expected model llama2, got %v", reqBody["model"])
		}
		if reqBody["prompt"] != "Hello, world!" {
			t.Errorf("expected prompt 'Hello, world!', got %v", reqBody["prompt"])
		}
		if reqBody["stream"] != false {
			t.Errorf("expected stream false, got %v", reqBody["stream"])
		}

		response := map[string]interface{}{
			"response": "Hello! How can I help you today?",
			"done":     true,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewOllamaClient(server.URL, "llama2")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := client.Send(ctx, "Hello, world!")
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	expected := "Hello! How can I help you today?"
	if result != expected {
		t.Errorf("expected response %q, got %q", expected, result)
	}
}

func TestOllamaClient_Send_WithMockResponder(t *testing.T) {
	client := NewOllamaClient("", "llama2")
	client.WithMockResponder(func(prompt string) (string, error) {
		return "Mock response for: " + prompt, nil
	})

	ctx := context.Background()
	result, err := client.Send(ctx, "test prompt")
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	expected := "Mock response for: test prompt"
	if result != expected {
		t.Errorf("expected response %q, got %q", expected, result)
	}
}

func TestOllamaClient_Send_ErrorHandling(t *testing.T) {
	// Test with empty model
	client := NewOllamaClient("", "")
	ctx := context.Background()

	_, err := client.Send(ctx, "test")
	if err == nil {
		t.Error("expected error for empty model, got nil")
	}

	// Test with API error response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"error": "model not found",
			"done":  false,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client = NewOllamaClient(server.URL, "nonexistent")
	_, err = client.Send(ctx, "test")
	if err == nil {
		t.Error("expected error for API error response, got nil")
	}
	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}

	// Test with HTTP error
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	client = NewOllamaClient(server.URL, "llama2")
	_, err = client.Send(ctx, "test")
	if err == nil {
		t.Error("expected error for HTTP 500, got nil")
	}
}

func TestOllamaClient_Send_ContextCancellation(t *testing.T) {
	// Create a server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		response := map[string]interface{}{
			"response": "delayed response",
			"done":     true,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewOllamaClient(server.URL, "llama2")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := client.Send(ctx, "test")
	if err == nil {
		t.Error("expected error for context cancellation, got nil")
	}
	// Check if error is context deadline exceeded (may be wrapped)
	errMsg := err.Error()
	if err != context.DeadlineExceeded && !strings.Contains(errMsg, "context deadline exceeded") {
		t.Errorf("expected context deadline exceeded error, got %v", err)
	}
}

// TestOllamaProvider_Integration verifies the full feature workflow:
// Step 1: Configure a local Ollama service (mock server)
// Step 2: Set the agent provider to 'ollama' and specify a model profile
// Step 3: Run the agent and verify it successfully communicates with the local Ollama instance
func TestOllamaProvider_Integration(t *testing.T) {
	// Step 1: Configure a local Ollama service (mock server)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/api/generate" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		// Verify model is specified
		model, ok := reqBody["model"].(string)
		if !ok || model == "" {
			t.Error("expected model to be specified in request")
		}

		response := map[string]interface{}{
			"response": "Hello from Ollama! Model: " + model,
			"done":     true,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Step 2: Set the agent provider to 'ollama' and specify a model profile
	agentClient, err := NewAgent("ollama", server.URL, "mistral")
	if err != nil {
		t.Fatalf("failed to create Ollama agent: %v", err)
	}

	ollamaClient, ok := agentClient.(*OllamaClient)
	if !ok {
		t.Fatalf("expected *OllamaClient, got %T", agentClient)
	}

	if ollamaClient.model != "mistral" {
		t.Errorf("expected model 'mistral', got %s", ollamaClient.model)
	}

	// Step 3: Run the agent and verify it successfully communicates with the local Ollama instance
	ctx := context.Background()
	result, err := agentClient.Send(ctx, "Hello, Ollama!")
	if err != nil {
		t.Fatalf("failed to communicate with Ollama: %v", err)
	}

	expectedPrefix := "Hello from Ollama! Model: mistral"
	if result != expectedPrefix {
		t.Errorf("expected response starting with %q, got %q", expectedPrefix, result)
	}
}
