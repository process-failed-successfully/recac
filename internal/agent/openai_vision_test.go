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

// MockTransport is already defined in gemini_vision_test.go

func TestOpenAIClient_SendImage(t *testing.T) {
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
			if !strings.Contains(req.URL.String(), "chat/completions") {
				t.Errorf("Unexpected URL: %s", req.URL.String())
			}

			// Verify Body
			bodyBytes, _ := io.ReadAll(req.Body)
			var bodyMap map[string]interface{}
			json.Unmarshal(bodyBytes, &bodyMap)

			messages := bodyMap["messages"].([]interface{})
			content := messages[0].(map[string]interface{})["content"].([]interface{})

			// Check text prompt
			textPart := content[0].(map[string]interface{})
			if textPart["text"] != "Describe this image" {
				t.Errorf("Expected text prompt 'Describe this image', got %v", textPart["text"])
			}

			// Check image part
			imagePart := content[1].(map[string]interface{})
			imageUrl := imagePart["image_url"].(map[string]interface{})
			if imageUrl["url"] == "" {
				t.Error("Expected image_url to be set")
			}
			if !strings.HasPrefix(imageUrl["url"].(string), "data:") || !strings.Contains(imageUrl["url"].(string), ";base64,") {
				t.Errorf("Expected base64 data uri, got %s", imageUrl["url"])
			}

			// Return mock response
			responseBody := `{
				"choices": [
					{
						"message": {
							"content": "This is a description of the image."
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

	client := NewOpenAIClient("dummy-key", "gpt-4-vision-preview", "test-project")
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
