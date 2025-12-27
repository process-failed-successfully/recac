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

// OllamaClient implements the Agent interface for local Ollama service
type OllamaClient struct {
	baseURL    string
	model      string
	httpClient *http.Client
	// mockResponder is used for testing to bypass real API calls
	mockResponder func(string) (string, error)
}

// NewOllamaClient creates a new Ollama client
// baseURL defaults to http://localhost:11434 if empty
// model is the Ollama model name (e.g., "llama2", "mistral", "codellama")
func NewOllamaClient(baseURL, model string) *OllamaClient {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	return &OllamaClient{
		baseURL: baseURL,
		model:   model,
		httpClient: &http.Client{
			Timeout: 60 * time.Second, // Longer timeout for local models
		},
	}
}

// WithMockResponder sets a mock responder for testing
func (c *OllamaClient) WithMockResponder(fn func(string) (string, error)) *OllamaClient {
	c.mockResponder = fn
	return c
}

// Send sends a prompt to Ollama and returns the generated text
func (c *OllamaClient) Send(ctx context.Context, prompt string) (string, error) {
	// Use mock responder if set (for testing)
	if c.mockResponder != nil {
		return c.mockResponder(prompt)
	}

	if c.model == "" {
		return "", fmt.Errorf("model is required for Ollama")
	}

	// Ollama API endpoint
	apiURL := fmt.Sprintf("%s/api/generate", c.baseURL)

	// Ollama request format
	requestBody := map[string]interface{}{
		"model":  c.model,
		"prompt": prompt,
		"stream": false, // We want a complete response, not streaming
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request to Ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Ollama API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Ollama response format
	var response struct {
		Response string `json:"response"`
		Done     bool   `json:"done"`
		Error    string `json:"error,omitempty"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if response.Error != "" {
		return "", fmt.Errorf("Ollama API error: %s", response.Error)
	}

	if !response.Done {
		return "", fmt.Errorf("Ollama response incomplete")
	}

	return response.Response, nil
}
