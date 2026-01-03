package agent

import (
	"context"
	"path/filepath"
	"testing"
)

func TestOpenAIClient_Mock(t *testing.T) {
	client := NewOpenAIClient("test-key", "gpt-4", "test-project")
	client.WithMockResponder(func(prompt string) (string, error) {
		return "mock response", nil
	})

	resp, err := client.Send(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if resp != "mock response" {
		t.Errorf("Expected 'mock response', got '%s'", resp)
	}
}

func TestOpenAIClient_StateTracking(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(filepath.Join(tmpDir, "state.json"))

	client := NewOpenAIClient("test-key", "gpt-4", "test-project")
	client.WithMockResponder(func(prompt string) (string, error) {
		return "mock response", nil
	})
	client.WithStateManager(sm)

	if _, err := client.Send(context.Background(), "hello"); err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	state, _ := sm.Load()
	if state.TokenUsage.TotalPromptTokens == 0 {
		t.Error("Expected token usage tracking")
	}
}

func TestOpenAIClient_NoKey(t *testing.T) {
	client := NewOpenAIClient("", "gpt-4", "test-project")
	// No mock responder -> sendOnce should fail check

	_, err := client.Send(context.Background(), "hello")
	if err == nil {
		t.Error("Expected error for missing API key")
	}
}
