package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestGeminiClient_SendImage(t *testing.T) {
	// Create a dummy image file
	tmpDir := t.TempDir()
	imagePath := filepath.Join(tmpDir, "test.png")
	err := os.WriteFile(imagePath, []byte("fake-image-data"), 0644)
	if err != nil {
		t.Fatalf("Failed to create dummy image: %v", err)
	}

	expectedResponse := "This is a vision response"
	client := NewGeminiClient("dummy-key", "gemini-pro-vision", "test-project").WithMockResponder(func(prompt string) (string, error) {
		// Mock responder just returns the text
		// In SendImage, we don't pass the image data to the mock responder string, only the prompt.
		if prompt == "Describe this image" {
			return expectedResponse, nil
		}
		return "", nil
	})

	resp, err := client.SendImage(context.Background(), "Describe this image", imagePath)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if resp != expectedResponse {
		t.Errorf("Expected response %q, got %q", expectedResponse, resp)
	}
}

func TestGeminiClient_SendImage_InvalidFile(t *testing.T) {
	client := NewGeminiClient("dummy-key", "gemini-pro-vision", "test-project")

	_, err := client.SendImage(context.Background(), "prompt", "non-existent.png")
	if err == nil {
		t.Error("Expected error for missing file, got nil")
	}
}
