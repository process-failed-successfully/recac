package agent

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSendImageOnce(t *testing.T) {
	expectedResponse := "This is a cat."
	expectedPrompt := "What is this?"
	expectedImage := "data:image/jpeg;base64,aabbcc"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify method
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		// Verify body
		body, _ := io.ReadAll(r.Body)
		var reqBody map[string]interface{}
		json.Unmarshal(body, &reqBody)

		messages := reqBody["messages"].([]interface{})
		content := messages[0].(map[string]interface{})["content"].([]interface{})

		textPart := content[0].(map[string]interface{})
		if textPart["type"] != "text" || textPart["text"] != expectedPrompt {
			t.Errorf("Unexpected text part: %v", textPart)
		}

		imagePart := content[1].(map[string]interface{})
		if imagePart["type"] != "image_url" {
			t.Errorf("Unexpected image part type: %v", imagePart["type"])
		}

		imageURL := imagePart["image_url"].(map[string]interface{})
		if imageURL["url"] != expectedImage {
			t.Errorf("Expected image url %s, got %v", expectedImage, imageURL["url"])
		}

		// Send response
		response := map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content": expectedResponse,
					},
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer ts.Close()

	cfg := HTTPClientConfig{
		APIKey:     "test-key",
		Model:      "gpt-4-vision",
		APIURL:     ts.URL,
		HTTPClient: ts.Client(),
	}

	resp, err := SendImageOnce(context.Background(), cfg, expectedPrompt, expectedImage)
	if err != nil {
		t.Fatalf("SendImageOnce failed: %v", err)
	}

	if resp != expectedResponse {
		t.Errorf("Expected response %s, got %s", expectedResponse, resp)
	}
}
