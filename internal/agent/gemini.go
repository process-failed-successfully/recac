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

// GeminiClient implements the Agent interface for Google's Gemini
type GeminiClient struct {
	apiKey     string
	model      string
	project    string
	httpClient *http.Client
	apiURL     string
	// mockResponder is used for testing to bypass real API calls
	mockResponder func(string) (string, error)
	// stateManager is optional; if set, enables token tracking and truncation
	stateManager *StateManager
	// backoffFn is used for exponential backoff (can be mocked for testing)
	backoffFn func(int) time.Duration
}

// NewGeminiClient creates a new Gemini client
func NewGeminiClient(apiKey, model, project string) *GeminiClient {
	return &GeminiClient{
		apiKey:  apiKey,
		model:   model,
		project: project,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		apiURL: "https://generativelanguage.googleapis.com/v1beta/models",
		backoffFn: func(retry int) time.Duration {
			return time.Duration(1<<uint(retry-1)) * time.Second
		},
	}
}

// WithMockResponder sets a mock responder for testing
func (c *GeminiClient) WithMockResponder(fn func(string) (string, error)) *GeminiClient {
	c.mockResponder = fn
	return c
}

// WithStateManager sets the state manager for token tracking
func (c *GeminiClient) WithStateManager(sm *StateManager) *GeminiClient {
	c.stateManager = sm
	return c
}

// Send sends a prompt to Gemini and returns the generated text with retry logic.
// If stateManager is configured, it will track tokens and truncate if needed.
func (c *GeminiClient) Send(ctx context.Context, prompt string) (string, error) {
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
			maxTokens = 32000 // Default to 32k if not set
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
		state.TokenUsage.TotalPromptTokens += promptTokens
		telemetry.TrackTokenUsage(c.project, promptTokens)

		// Log token usage
		telemetry.LogDebug("Token usage (prompt)",
			"project", c.project,
			"prompt", promptTokens,
			"current", state.CurrentTokens,
			"max", maxTokens,
			"total_prompt", state.TokenUsage.TotalPromptTokens,
			"truncations", state.TokenUsage.TruncationCount)
	}

	maxRetries := 3
	var lastErr error

	for i := 0; i <= maxRetries; i++ {
		if i > 0 {
			// Exponential backoff
			waitTime := c.backoffFn(i)
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
				state.TokenUsage.TotalResponseTokens += responseTokens
				state.TokenUsage.TotalTokens = state.TokenUsage.TotalPromptTokens + state.TokenUsage.TotalResponseTokens
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
					maxTokens = 32000
				}
				telemetry.LogInfo("Token usage (response)",
					"project", c.project,
					"response", responseTokens,
					"current", state.CurrentTokens,
					"max", maxTokens,
					"total", state.TokenUsage.TotalTokens,
					"prompt", state.TokenUsage.TotalPromptTokens,
					"response_total", state.TokenUsage.TotalResponseTokens)

				// Save updated state
				if err := c.stateManager.Save(state); err != nil {
					fmt.Printf("Warning: Failed to save state: %v\n", err)
				}
			}
			return result, nil
		}

		lastErr = err
	}

	return "", fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
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
	resp, err := c.Send(ctx, prompt)
	if err == nil && onChunk != nil {
		onChunk(resp)
	}
	return resp, err
}
