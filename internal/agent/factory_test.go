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
