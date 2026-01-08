package agent

import (
	"context"
	"os"
	"testing"
)

func TestOllamaClient_StateTracking(t *testing.T) {
	// Setup temporary directory for state
	tmpDir, err := os.MkdirTemp("", "ollama_state_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create StateManager
	// NewStateManager expects a file path, not a directory
	stateFile := tmpDir + "/state.json"
	sm := NewStateManager(stateFile)
	sm.Save(State{
		MaxTokens: 1000,
	})

	// Create client with mock responder
	client := NewOllamaClient("http://localhost:11434", "test-model", "test-project")
	client.WithStateManager(sm)
	client.WithMockResponder(func(prompt string) (string, error) {
		return "This is a response", nil
	})

	// Send request
	resp, err := client.Send(context.Background(), "Hello")
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if resp != "This is a response" {
		t.Errorf("Expected response 'This is a response', got '%s'", resp)
	}

	// Verify state was updated
	state, err := sm.Load()
	if err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}

	if state.TokenUsage.PromptTokens == 0 {
		t.Error("PromptTokens should be > 0")
	}
	if state.TokenUsage.CompletionTokens == 0 {
		t.Error("CompletionTokens should be > 0")
	}
	if state.Metadata["iteration"] != 1.0 {
		t.Errorf("Expected iteration 1.0, got %v", state.Metadata["iteration"])
	}
}
