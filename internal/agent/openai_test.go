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

func TestOpenAIClient_SendImage(t *testing.T) {
	expectedResponse := "This is a dog"
	imageData := []byte("fake image data")

	client := NewOpenAIClient("dummy-key", "gpt-4-vision-preview", "test-project").
		WithMockImageResponder(func(prompt string, img []byte) (string, error) {
			if prompt != "Describe this" {
				t.Errorf("Expected prompt 'Describe this', got %q", prompt)
			}
			if string(img) != string(imageData) {
				t.Errorf("Expected image data %q, got %q", imageData, img)
			}
			return expectedResponse, nil
		})

	resp, err := client.SendImage(context.Background(), "Describe this", imageData)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if resp != expectedResponse {
		t.Errorf("Expected response %q, got %q", expectedResponse, resp)
	}
}
