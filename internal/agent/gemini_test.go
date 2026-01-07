package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGeminiClient_MockResponse(t *testing.T) {
	expectedResponse := "This is a mock response"
	client := NewGeminiClient("dummy-key", "gemini-pro", "test-project").WithMockResponder(func(prompt string) (string, error) {
		if prompt == "Hello" {
			return expectedResponse, nil
		}
		return "", nil
	})

	resp, err := client.Send(context.Background(), "Hello")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if resp != expectedResponse {
		t.Errorf("Expected response %q, got %q", expectedResponse, resp)
	}
}

func TestGeminiClient_RealCallValidation(t *testing.T) {
	// Without mock responder, it should fail if API key is missing
	client := NewGeminiClient("", "gemini-pro", "test-project")
	client.BackoffFn = func(i int) time.Duration { return time.Millisecond }
	_, err := client.Send(context.Background(), "Hello")
	if err == nil {
		t.Error("Expected error for missing API key, got nil")
	}
}

func TestGeminiClient_TokenTracking(t *testing.T) {
	// Create temporary state file
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, ".agent_state.json")
	stateManager := NewStateManager(stateFile)

	// Create a large prompt that will exceed token limit
	largePrompt := ""
	for i := 0; i < 1000; i++ {
		largePrompt += "This is a very long sentence that will contribute to the token count. "
	}

	expectedResponse := "Mock response"
	client := NewGeminiClient("dummy-key", "gemini-pro", "test-project").
		WithStateManager(stateManager).
		WithMockResponder(func(prompt string) (string, error) {
			// Verify prompt was truncated
			promptTokens := EstimateTokenCount(prompt)
			if promptTokens > 16000 { // Should be truncated to ~50% of 32k default
				t.Errorf("Prompt should be truncated, but has %d tokens", promptTokens)
			}
			return expectedResponse, nil
		})

	// Set a low token limit to force truncation
	state, _ := stateManager.Load()
	state.MaxTokens = 1000 // Very low limit
	stateManager.Save(state)

	resp, err := client.Send(context.Background(), largePrompt)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if resp != expectedResponse {
		t.Errorf("Expected response %q, got %q", expectedResponse, resp)
	}

	// Verify state was updated with token usage
	updatedState, _ := stateManager.Load()
	if updatedState.TokenUsage.TotalPromptTokens == 0 {
		t.Error("Expected prompt tokens to be tracked, got 0")
	}
	if updatedState.TokenUsage.TotalResponseTokens == 0 {
		t.Error("Expected response tokens to be tracked, got 0")
	}
	if updatedState.TokenUsage.TruncationCount == 0 {
		t.Error("Expected truncation count to be incremented, got 0")
	}
}

func TestGeminiClient_TokenTrackingNoTruncation(t *testing.T) {
	// Create temporary state file
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, ".agent_state.json")
	stateManager := NewStateManager(stateFile)

	// Create a small prompt that won't exceed token limit
	smallPrompt := "Hello, this is a short prompt."

	expectedResponse := "Mock response"
	client := NewGeminiClient("dummy-key", "gemini-pro", "test-project").
		WithStateManager(stateManager).
		WithMockResponder(func(prompt string) (string, error) {
			// Verify prompt was NOT truncated
			if prompt != smallPrompt {
				t.Errorf("Prompt should not be truncated, got %q", prompt)
			}
			return expectedResponse, nil
		})

	resp, err := client.Send(context.Background(), smallPrompt)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if resp != expectedResponse {
		t.Errorf("Expected response %q, got %q", expectedResponse, resp)
	}

	// Verify state was updated with token usage
	updatedState, _ := stateManager.Load()
	if updatedState.TokenUsage.TotalPromptTokens == 0 {
		t.Error("Expected prompt tokens to be tracked, got 0")
	}
	if updatedState.TokenUsage.TotalResponseTokens == 0 {
		t.Error("Expected response tokens to be tracked, got 0")
	}
	if updatedState.TokenUsage.TruncationCount != 0 {
		t.Errorf("Expected no truncation, but truncation count is %d", updatedState.TokenUsage.TruncationCount)
	}

	// Cleanup
	os.Remove(stateFile)
}
