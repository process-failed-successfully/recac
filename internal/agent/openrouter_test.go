package agent

import (
	"context"
	"testing"
)

func TestOpenRouterClient_Send(t *testing.T) {
	client := NewOpenRouterClient("test-key", "google/gemini-2.0-flash-001", "test-project")
	client.WithMockResponder(func(prompt string) (string, error) {
		if prompt == "Hello" {
			return "World", nil
		}
		return "", nil
	})

	resp, err := client.Send(context.Background(), "Hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp != "World" {
		t.Errorf("expected 'World', got '%s'", resp)
	}
}

func TestOpenRouterClient_New(t *testing.T) {
	client := NewOpenRouterClient("api-key", "model-id", "test-project")
	if client.apiKey != "api-key" {
		t.Error("API key not set")
	}
	if client.model != "model-id" {
		t.Error("Model not set")
	}
	if client.apiURL != "https://openrouter.ai/api/v1/chat/completions" {
		t.Error("Wrong API URL")
	}
}
