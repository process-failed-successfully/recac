package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"recac/internal/telemetry"
	"time"
)

// OllamaClient implements the Agent interface for local Ollama service
type OllamaClient struct {
	baseURL    string
	model      string
	project    string
	httpClient *http.Client
	// mockResponder is used for testing to bypass real API calls
	mockResponder func(string) (string, error)
	// stateManager is optional; if set, enables token tracking and truncation
	stateManager *StateManager
}

// NewOllamaClient creates a new Ollama client
// baseURL defaults to http://localhost:11434 if empty
// model is the Ollama model name (e.g., "llama2", "mistral", "codellama")
func NewOllamaClient(baseURL, model, project string) *OllamaClient {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	return &OllamaClient{
		baseURL: baseURL,
		model:   model,
		project: project,
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

// WithStateManager sets the state manager for token tracking
func (c *OllamaClient) WithStateManager(sm *StateManager) *OllamaClient {
	c.stateManager = sm
	return c
}

// Send sends a prompt to Ollama and returns the generated text
func (c *OllamaClient) Send(ctx context.Context, prompt string) (string, error) {
	telemetry.TrackAgentIteration(c.project)
	start := time.Now()
	defer func() {
		telemetry.ObserveAgentLatency(c.project, time.Since(start).Seconds())
	}()

	// Load state and check token limits if state manager is configured
	var state State
	var shouldUpdateState bool
	if c.stateManager != nil {
		var err error
		state, err = c.stateManager.Load()
		if err != nil {
			return "", fmt.Errorf("failed to load state: %w", err)
		}
		shouldUpdateState = true

		// Check if prompt exceeds token limit
		promptTokens := EstimateTokenCount(prompt)
		maxTokens := state.MaxTokens
		if maxTokens == 0 {
			maxTokens = 8192 // Default to 8k for local models if not set
		}

		// Reserve some tokens for response (estimate 50% for response)
		availableTokens := maxTokens * 50 / 100
		if promptTokens > availableTokens {
			// Truncate the prompt
			telemetry.LogInfo("Prompt exceeds token limit, truncating...", "project", c.project, "actual", promptTokens, "available", availableTokens)
			prompt = TruncateToTokenLimit(prompt, availableTokens)
			promptTokens = EstimateTokenCount(prompt)
			state.TokenUsage.TruncationCount++
		}

		// Update current token count
		state.CurrentTokens = promptTokens
		state.TokenUsage.PromptTokens += promptTokens
		telemetry.TrackTokenUsage(c.project, promptTokens)

		// Log token usage
		telemetry.LogDebug("Token usage (prompt)",
			"project", c.project,
			"prompt", promptTokens,
			"current", state.CurrentTokens,
			"max", maxTokens,
			"total_prompt", state.TokenUsage.PromptTokens,
			"truncations", state.TokenUsage.TruncationCount)
	}

	maxRetries := 3
	var lastErr error

	for i := 0; i <= maxRetries; i++ {
		if i > 0 {
			// Exponential backoff
			waitTime := time.Duration(1<<uint(i-1)) * time.Second
			telemetry.LogInfo("Retrying agent call", "project", c.project, "retry", i, "wait", waitTime, "error", lastErr)
			select {
			case <-time.After(waitTime):
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}

		result, err := c.sendOnce(ctx, prompt)
		if err == nil {
			// Update token usage stats if state manager is configured
			if shouldUpdateState {
				responseTokens := EstimateTokenCount(result)
				state.TokenUsage.CompletionTokens += responseTokens
				state.TokenUsage.TotalTokens = state.TokenUsage.PromptTokens + state.TokenUsage.CompletionTokens
				state.CurrentTokens += responseTokens

				// Initialize Metadata if needed
				if state.Metadata == nil {
					state.Metadata = make(map[string]interface{})
				}

				// Increment iteration count only on successful calls
				currentIteration, _ := state.Metadata["iteration"].(float64)
				state.Metadata["iteration"] = currentIteration + 1

				// Log token usage after response
				maxTokens := state.MaxTokens
				if maxTokens == 0 {
					maxTokens = 8192
				}
				telemetry.LogInfo("Token usage (response)",
					"project", c.project,
					"response", responseTokens,
					"current", state.CurrentTokens,
					"max", maxTokens,
					"total", state.TokenUsage.TotalTokens,
					"prompt", state.TokenUsage.PromptTokens,
					"response_total", state.TokenUsage.CompletionTokens)

				// Save updated state
				if err := c.stateManager.Save(state); err != nil {
					telemetry.LogInfo("Warning: Failed to save state", "project", c.project, "error", err)
				}
			}
			return result, nil
		}

		lastErr = err
	}

	return "", fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

func (c *OllamaClient) sendOnce(ctx context.Context, prompt string) (string, error) {
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

// SendStream fallback for Ollama (calls Send and emits once)
func (c *OllamaClient) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	resp, err := c.Send(ctx, prompt)
	if err == nil && onChunk != nil {
		onChunk(resp)
	}
	return resp, err
}
