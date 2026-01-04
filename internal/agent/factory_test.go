package agent

import (
	"testing"
)

func TestNewAgent(t *testing.T) {
	// Test Gemini
	a, err := NewAgent("gemini", "key", "gemini-pro", "", "test-project")
	if err != nil {
		t.Fatalf("failed to create gemini agent: %v", err)
	}
	if _, ok := a.(*GeminiClient); !ok {
		t.Errorf("expected *GeminiClient, got %T", a)
	}

	// Test OpenAI
	a, err = NewAgent("openai", "key", "gpt-4", "", "test-project")
	if err != nil {
		t.Fatalf("failed to create openai agent: %v", err)
	}
	if _, ok := a.(*OpenAIClient); !ok {
		t.Errorf("expected *OpenAIClient, got %T", a)
	}

	// Test Ollama
	a, err = NewAgent("ollama", "", "llama2", "", "test-project")
	if err != nil {
		t.Fatalf("failed to create ollama agent: %v", err)
	}
	if _, ok := a.(*OllamaClient); !ok {
		t.Errorf("expected *OllamaClient, got %T", a)
	}

	// Test Unknown
	_, err = NewAgent("unknown", "key", "model", "", "test-project")
	if err == nil {
		t.Error("expected error for unknown provider, got nil")
	}
}

func TestNewAgent_OpenRouterModelCorrection(t *testing.T) {
	tests := []struct {
		model    string
		expected string
	}{
		{"gemini-pro-latest", "google/gemini-pro-latest"},
		{"gpt-4-turbo", "openai/gpt-4-turbo"},
		{"claude-3-opus", "anthropic/claude-3-opus"},
		{"llama-3-70b", "meta-llama/llama-3-70b"},
		{"mistral-large", "mistralai/mistral-large"},
		{"mixtral-8x7b", "mistralai/mixtral-8x7b"},
		{"google/gemini-pro-latest", "google/gemini-pro-latest"}, // Already prefixed
		{"custom-model", "custom-model"},                         // No prefix match
	}

	for _, tt := range tests {
		a, err := NewAgent("openrouter", "key", tt.model, "", "test-project")
		if err != nil {
			t.Fatalf("failed to create openrouter agent for %s: %v", tt.model, err)
		}
		client, ok := a.(*OpenRouterClient)
		if !ok {
			t.Fatalf("expected *OpenRouterClient, got %T", a)
		}
		if client.model != tt.expected {
			t.Errorf("for model %s, expected %s, got %s", tt.model, tt.expected, client.model)
		}
	}
}
