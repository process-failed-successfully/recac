package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// GeminiClient implements the Agent interface for Google's Gemini
type GeminiClient struct {
	BaseClient
	apiKey     string
	model      string
	httpClient *http.Client
	apiURL     string
	// mockResponder is used for testing to bypass real API calls
	mockResponder func(string) (string, error)
}

// NewGeminiClient creates a new Gemini client
func NewGeminiClient(apiKey, model, project string) *GeminiClient {
	return &GeminiClient{
		BaseClient: NewBaseClient(project, 32000), // Default to 32k for Gemini
		apiKey:     apiKey,
		model:      model,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		apiURL: "https://generativelanguage.googleapis.com/v1beta/models",
	}
}

// WithMockResponder sets a mock responder for testing
func (c *GeminiClient) WithMockResponder(fn func(string) (string, error)) *GeminiClient {
	c.mockResponder = fn
	return c
}

// WithStateManager sets the state manager for token tracking
func (c *GeminiClient) WithStateManager(sm *StateManager) *GeminiClient {
	c.StateManager = sm
	return c
}

// Send sends a prompt to Gemini and returns the generated text with retry logic.
// If stateManager is configured, it will track tokens and truncate if needed.
func (c *GeminiClient) Send(ctx context.Context, prompt string) (string, error) {
	return c.SendWithRetry(ctx, prompt, c.sendOnce)
}

func (c *GeminiClient) sendOnce(ctx context.Context, prompt string) (string, error) {
	if c.mockResponder != nil {
		return c.mockResponder(prompt)
	}

	if c.apiKey == "" {
		return "", fmt.Errorf("API key is required")
	}

	url := fmt.Sprintf("%s/%s:generateContent", c.apiURL, c.model)

	requestBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]interface{}{
					{"text": prompt},
				},
			},
		},
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", c.apiKey)

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
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(response.Candidates) == 0 || len(response.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no content in response")
	}

	return response.Candidates[0].Content.Parts[0].Text, nil
}

// SendStream fallback for Gemini (calls Send and emits once)
func (c *GeminiClient) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return c.SendStreamWithRetry(ctx, prompt, func(ctx context.Context, p string, oc func(string)) (string, error) {
		// Mock streaming by calling SendOnce and emitting the whole result
		resp, err := c.sendOnce(ctx, p)
		if err == nil && oc != nil {
			oc(resp)
		}
		return resp, err
	}, onChunk)
}
