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
	*BaseClient
	baseURL       string
	model         string
	httpClient    *http.Client
	mockResponder func(string) (string, error)
}

// NewOllamaClient creates a new Ollama client
func NewOllamaClient(baseURL, model, project string) *OllamaClient {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	client := &OllamaClient{
		baseURL:    baseURL,
		model:      model,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
	client.BaseClient = NewBaseClient(project, nil, client)
	return client
}

// WithMockResponder sets a mock responder for testing
func (c *OllamaClient) WithMockResponder(fn func(string) (string, error)) *OllamaClient {
	c.mockResponder = fn
	return c
}

// WithStateManager sets the state manager for token tracking
func (c *OllamaClient) WithStateManager(sm *StateManager) *OllamaClient {
	c.stateManager = sm
	return c
}

func (c *OllamaClient) getDefaultMaxTokens() int {
	return 8192 // Default to 8k for local models
}

func (c *OllamaClient) sendOnce(ctx context.Context, prompt string) (string, error) {
	if c.mockResponder != nil {
		return c.mockResponder(prompt)
	}

	if c.model == "" {
		return "", fmt.Errorf("model is required for Ollama")
	}

	apiURL := fmt.Sprintf("%s/api/generate", c.baseURL)

	requestBody := map[string]interface{}{
		"model":  c.model,
		"prompt": prompt,
		"stream": false,
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

// SendStream fallback for Ollama (calls Send and emits once)
func (c *OllamaClient) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	resp, err := c.Send(ctx, prompt)
	if err == nil && onChunk != nil {
		onChunk(resp)
	}
	return resp, err
}
