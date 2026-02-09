package agent

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OpenAIClient represents a client for the OpenAI API
type OpenAIClient struct {
	BaseClient
	apiKey     string
	model      string
	httpClient *http.Client
	apiURL     string
	// mockResponder is used for testing to bypass real API calls
	mockResponder      func(string) (string, error)
	mockImageResponder func(string, []byte) (string, error)
}

// NewOpenAIClient creates a new OpenAI client
func NewOpenAIClient(apiKey, model, project string) *OpenAIClient {
	return &OpenAIClient{
		BaseClient: NewBaseClient(project, 128000), // Default to 128k for GPT-4
		apiKey:     apiKey,
		model:      model,
		httpClient: &http.Client{
			Timeout: 300 * time.Second,
		},
		apiURL: "https://api.openai.com/v1/chat/completions",
	}
}

// WithMockResponder sets a mock responder for testing
func (c *OpenAIClient) WithMockResponder(fn func(string) (string, error)) *OpenAIClient {
	c.mockResponder = fn
	return c
}

// WithMockImageResponder sets a mock image responder for testing
func (c *OpenAIClient) WithMockImageResponder(fn func(string, []byte) (string, error)) *OpenAIClient {
	c.mockImageResponder = fn
	return c
}

// WithStateManager sets the state manager for token tracking
func (c *OpenAIClient) WithStateManager(sm *StateManager) *OpenAIClient {
	c.StateManager = sm
	return c
}

func (c *OpenAIClient) getConfig() HTTPClientConfig {
	return HTTPClientConfig{
		BaseClient:    &c.BaseClient,
		APIKey:        c.apiKey,
		Model:         c.model,
		APIURL:        c.apiURL,
		HTTPClient:    c.httpClient,
		MockResponder: c.mockResponder,
	}
}

// Send sends a prompt to OpenAI and returns the generated text with retry logic.
// If stateManager is configured, it will track tokens and truncate if needed.
func (c *OpenAIClient) Send(ctx context.Context, prompt string) (string, error) {
	return c.SendWithRetry(ctx, prompt, c.sendOnce)
}

func (c *OpenAIClient) sendOnce(ctx context.Context, prompt string) (string, error) {
	return SendOnce(ctx, c.getConfig(), prompt)
}

// SendStream sends a prompt to OpenAI and streams the response
func (c *OpenAIClient) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return c.SendStreamWithRetry(ctx, prompt, func(ctx context.Context, p string, oc func(string)) (string, error) {
		return SendStreamOnce(ctx, c.getConfig(), p, oc)
	}, onChunk)
}

// SendImage sends a prompt with an image to OpenAI
func (c *OpenAIClient) SendImage(ctx context.Context, prompt string, image []byte) (string, error) {
	return c.SendWithRetry(ctx, prompt, func(ctx context.Context, p string) (string, error) {
		if c.mockImageResponder != nil {
			return c.mockImageResponder(p, image)
		}

		if c.apiKey == "" {
			return "", fmt.Errorf("API key is required")
		}

		encodedImage := base64.StdEncoding.EncodeToString(image)
		mimeType := http.DetectContentType(image)
		dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, encodedImage)

		requestBody := map[string]interface{}{
			"model": c.model,
			"messages": []map[string]interface{}{
				{
					"role": "user",
					"content": []map[string]interface{}{
						{
							"type": "text",
							"text": p,
						},
						{
							"type": "image_url",
							"image_url": map[string]string{
								"url": dataURI,
							},
						},
					},
				},
			},
		}

		jsonBody, err := json.Marshal(requestBody)
		if err != nil {
			return "", fmt.Errorf("failed to marshal request: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, "POST", c.apiURL, bytes.NewBuffer(jsonBody))
		if err != nil {
			return "", fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.apiKey)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return "", fmt.Errorf("failed to send request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			return "", fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(bodyBytes))
		}

		var response struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return "", fmt.Errorf("failed to decode response: %w", err)
		}

		if len(response.Choices) == 0 {
			return "", fmt.Errorf("no content in response")
		}

		return response.Choices[0].Message.Content, nil
	})
}
