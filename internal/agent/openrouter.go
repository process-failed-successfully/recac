package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"recac/internal/telemetry"
)

// OpenRouterClient implements the Agent interface for OpenRouter
type OpenRouterClient struct {
	apiKey     string
	model      string
	project    string
	httpClient *http.Client
	apiURL     string
	// mockResponder is used for testing to bypass real API calls
	mockResponder func(string) (string, error)
	// stateManager is optional; if set, enables token tracking and truncation
	stateManager *StateManager
}

// NewOpenRouterClient creates a new OpenRouter client
func NewOpenRouterClient(apiKey, model, project string) *OpenRouterClient {
	return &OpenRouterClient{
		apiKey:  apiKey,
		model:   model,
		project: project,
		httpClient: &http.Client{
			Timeout: 300 * time.Second, // OpenRouter can be slower depending on the underlying model
		},
		apiURL: "https://openrouter.ai/api/v1/chat/completions",
	}
}

// WithMockResponder sets a mock responder for testing
func (c *OpenRouterClient) WithMockResponder(fn func(string) (string, error)) *OpenRouterClient {
	c.mockResponder = fn
	return c
}

// WithStateManager sets the state manager for token tracking
func (c *OpenRouterClient) WithStateManager(sm *StateManager) *OpenRouterClient {
	c.stateManager = sm
	return c
}

// Send sends a prompt to OpenRouter and returns the generated text
func (c *OpenRouterClient) Send(ctx context.Context, prompt string) (string, error) {
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
			maxTokens = 128000 // Default generic limit if not set
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
		if maxTokens > 0 {
			telemetry.SetContextUsage(c.project, float64(state.CurrentTokens)/float64(maxTokens)*100)
		}

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
					maxTokens = 128000
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
					telemetry.LogInfo("Warning: Failed to save state", "project", c.project, "error", err)
				}
			}
			return result, nil
		}

		lastErr = err
	}

	return "", fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

func (c *OpenRouterClient) sendOnce(ctx context.Context, prompt string) (string, error) {
	if c.mockResponder != nil {
		return c.mockResponder(prompt)
	}

	if c.apiKey == "" {
		return "", fmt.Errorf("API key is required")
	}

	requestBody := map[string]interface{}{
		"model": c.model,
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": prompt,
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
	req.Header.Set("HTTP-Referer", "https://github.com/process-failed-successfully/recac")
	req.Header.Set("X-Title", "Process Failed Successfully")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("OpenRouter API returned status %d: %s", resp.StatusCode, string(bodyBytes))
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
}

// SendStream sends a prompt to OpenRouter and streams the response
func (c *OpenRouterClient) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	telemetry.TrackAgentIteration(c.project)
	start := time.Now()
	defer func() {
		telemetry.ObserveAgentLatency(c.project, time.Since(start).Seconds())
	}()
	// Load state and check token limits
	var state State
	var shouldUpdateState bool
	if c.stateManager != nil {
		var err error
		state, err = c.stateManager.Load()
		if err != nil {
			return "", fmt.Errorf("failed to load state: %w", err)
		}
		shouldUpdateState = true

		promptTokens := EstimateTokenCount(prompt)
		maxTokens := state.MaxTokens
		if maxTokens == 0 {
			maxTokens = 128000
		}
		availableTokens := maxTokens * 50 / 100
		if promptTokens > availableTokens {
			telemetry.LogInfo("Prompt exceeds token limit, truncating...", "project", c.project, "actual", promptTokens, "available", availableTokens)
			prompt = TruncateToTokenLimit(prompt, availableTokens)
			promptTokens = EstimateTokenCount(prompt)
			state.TokenUsage.TruncationCount++
		}
		state.CurrentTokens = promptTokens
		state.TokenUsage.TotalPromptTokens += promptTokens
		telemetry.TrackTokenUsage(c.project, promptTokens)
		if maxTokens > 0 {
			telemetry.SetContextUsage(c.project, float64(state.CurrentTokens)/float64(maxTokens)*100)
		}
		telemetry.LogDebug("Token usage (prompt)",
			"project", c.project,
			"prompt", promptTokens,
			"current", state.CurrentTokens,
			"max", maxTokens,
			"total_prompt", state.TokenUsage.TotalPromptTokens,
			"truncations", state.TokenUsage.TruncationCount)
	}

	// Prepare Request
	requestBody := map[string]interface{}{
		"model":  c.model,
		"stream": true,
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": prompt,
			},
		},
	}

	// Stream Reading
	var fullResponse strings.Builder

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

		result, err := c.sendStreamOnce(ctx, prompt, requestBody, onChunk)
		if err == nil {
			fullResponse.WriteString(result)
			break
		}
		lastErr = err
	}

	if lastErr != nil {
		return "", fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
	}

	result := fullResponse.String()

	// Update Token Usage after streaming
	if shouldUpdateState {
		responseTokens := EstimateTokenCount(result)
		state.TokenUsage.TotalResponseTokens += responseTokens
		state.TokenUsage.TotalTokens = state.TokenUsage.TotalPromptTokens + state.TokenUsage.TotalResponseTokens
		state.CurrentTokens += responseTokens

		// Initialize Metadata if needed
		if state.Metadata == nil {
			state.Metadata = make(map[string]interface{})
		}
		currentIteration, _ := state.Metadata["iteration"].(float64)
		state.Metadata["iteration"] = currentIteration + 1

		maxTokens := state.MaxTokens
		if maxTokens == 0 {
			maxTokens = 128000
		}
		telemetry.LogInfo("Token usage (response)",
			"project", c.project,
			"response", responseTokens,
			"current", state.CurrentTokens,
			"max", maxTokens,
			"total", state.TokenUsage.TotalTokens,
			"prompt", state.TokenUsage.TotalPromptTokens,
			"response_total", state.TokenUsage.TotalResponseTokens)

		if err := c.stateManager.Save(state); err != nil {
			telemetry.LogInfo("Warning: Failed to save state", "project", c.project, "error", err)
		}
	}

	return result, nil
}

func (c *OpenRouterClient) sendStreamOnce(ctx context.Context, prompt string, requestBody map[string]interface{}, onChunk func(string)) (string, error) {
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
	req.Header.Set("HTTP-Referer", "https://github.com/process-failed-successfully/recac")
	req.Header.Set("X-Title", "Process Failed Successfully")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("OpenRouter API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var fullResponse strings.Builder
	reader := bufio.NewReader(resp.Body)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", fmt.Errorf("error reading stream: %w", err)
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var streamResp struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}

		if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
			continue // Skip malformed lines
		}

		if len(streamResp.Choices) > 0 {
			content := streamResp.Choices[0].Delta.Content
			if content != "" {
				fullResponse.WriteString(content)
				if onChunk != nil {
					onChunk(content)
				}
			}
		}
	}

	return fullResponse.String(), nil
}
