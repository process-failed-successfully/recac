package agent

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

// MockTransport allows mocking HTTP responses and capturing requests
type MockTransport struct {
	RoundTripFunc func(req *http.Request) (*http.Response, error)
}

func (m *MockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.RoundTripFunc(req)
}

func TestGeminiClient_SendImage(t *testing.T) {
	// Create a dummy image file
	tmpFile, err := os.CreateTemp("", "test_image.jpg")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	_, err = tmpFile.WriteString("dummy image content")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	// Mock HTTP client
	mockTransport := &MockTransport{
		RoundTripFunc: func(req *http.Request) (*http.Response, error) {
			// Verify URL
			if !strings.Contains(req.URL.String(), "generateContent") {
				t.Errorf("Unexpected URL: %s", req.URL.String())
			}

			// Verify Body
			bodyBytes, _ := io.ReadAll(req.Body)
			var bodyMap map[string]interface{}
			json.Unmarshal(bodyBytes, &bodyMap)

			contents := bodyMap["contents"].([]interface{})
			parts := contents[0].(map[string]interface{})["parts"].([]interface{})

			// Check text prompt
			textPart := parts[0].(map[string]interface{})
			if textPart["text"] != "Describe this image" {
				t.Errorf("Expected text prompt 'Describe this image', got %v", textPart["text"])
			}

			// Check image part
			imagePart := parts[1].(map[string]interface{})
			inlineData := imagePart["inline_data"].(map[string]interface{})
			if inlineData["mime_type"] == "" {
				t.Error("Expected mime_type to be set")
			}
			if inlineData["data"] == "" {
				t.Error("Expected base64 data to be set")
			}

			// Return mock response
			responseBody := `{
				"candidates": [
					{
						"content": {
							"parts": [
								{"text": "This is a description of the image."}
							]
						}
					}
				]
			}`

			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(responseBody)),
				Header:     make(http.Header),
			}, nil
		},
	}

	client := NewGeminiClient("dummy-key", "gemini-pro", "test-project")
	// Inject mock HTTP client
	client.httpClient = &http.Client{Transport: mockTransport}

	resp, err := client.SendImage(context.Background(), "Describe this image", tmpFile.Name())
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if resp != "This is a description of the image." {
		t.Errorf("Expected response 'This is a description of the image.', got %q", resp)
	}
}
