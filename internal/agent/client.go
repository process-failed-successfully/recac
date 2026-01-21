package agent

import (
	"context"
	"fmt"
	"recac/internal/telemetry"
	"time"
)

// BaseClient provides shared logic for all agent clients,
// including state management, token tracking, retry logic, and telemetry.
type BaseClient struct {
	Project      string
	StateManager *StateManager
	BackoffFn    func(int) time.Duration
	// DefaultMaxTokens is the default context limit if not set in state
	DefaultMaxTokens int
}

// NewBaseClient creates a new BaseClient
func NewBaseClient(project string, defaultMaxTokens int) BaseClient {
	return BaseClient{
		Project:          project,
		DefaultMaxTokens: defaultMaxTokens,
		BackoffFn: func(retry int) time.Duration {
			return time.Duration(1<<uint(retry-1)) * time.Second
		},
	}
}

// PreparePrompt checks token limits and truncates if necessary.
// Returns the (possibly truncated) prompt, the state, and a boolean indicating if state should be updated.
func (c *BaseClient) PreparePrompt(prompt string) (string, State, bool, error) {
	if c.StateManager == nil {
		return prompt, State{}, false, nil
	}

	state, err := c.StateManager.Load()
	if err != nil {
		return "", State{}, false, fmt.Errorf("failed to load state: %w", err)
	}

	// Check if prompt exceeds token limit
	promptTokens := EstimateTokenCount(prompt)
	maxTokens := state.MaxTokens
	if maxTokens == 0 {
		maxTokens = c.DefaultMaxTokens
		// If still 0, default to a safe value (32k) to avoid division by zero
		if maxTokens == 0 {
			maxTokens = 32000
		}
	}

	// Add message to history
	state.History = append(state.History, Message{
		Role:      "user",
		Content:   prompt,
		Timestamp: time.Now(),
	})

	// Reserve some tokens for response (estimate 50% for response)
	availableTokens := maxTokens * 50 / 100
	if promptTokens > availableTokens {
		// Truncate the prompt for the API call (but the history keeps the full or reasonably trimmed version)
		telemetry.LogInfo("Prompt exceeds token limit, truncating...", "project", c.Project, "actual", promptTokens, "available", availableTokens)
		prompt = TruncateToTokenLimit(prompt, availableTokens)
		promptTokens = EstimateTokenCount(prompt)
		state.TokenUsage.TruncationCount++
	}

	// Update current token count
	state.CurrentTokens = promptTokens
	state.TokenUsage.TotalPromptTokens += promptTokens
	telemetry.TrackTokenUsage(c.Project, promptTokens)

	// Log token usage
	telemetry.LogDebug("Token usage (prompt)",
		"project", c.Project,
		"prompt", promptTokens,
		"current", state.CurrentTokens,
		"max", maxTokens,
		"total_prompt", state.TokenUsage.TotalPromptTokens,
		"truncations", state.TokenUsage.TruncationCount)

	return prompt, state, true, nil
}

// UpdateStateWithResponse updates the state with response token usage and saves it.
func (c *BaseClient) UpdateStateWithResponse(state State, response string) {
	if c.StateManager == nil {
		return
	}

	responseTokens := EstimateTokenCount(response)
	state.TokenUsage.TotalResponseTokens += responseTokens
	state.TokenUsage.TotalTokens = state.TokenUsage.TotalPromptTokens + state.TokenUsage.TotalResponseTokens
	state.CurrentTokens += responseTokens
	telemetry.TrackTokenUsage(c.Project, responseTokens)

	// Add response to history
	state.History = append(state.History, Message{
		Role:      "assistant",
		Content:   response,
		Timestamp: time.Now(),
	})

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
		maxTokens = c.DefaultMaxTokens
		if maxTokens == 0 {
			maxTokens = 32000
		}
	}

	if maxTokens > 0 {
		telemetry.SetContextUsage(c.Project, float64(state.CurrentTokens)/float64(maxTokens)*100)
	}

	telemetry.LogInfo("Token usage (response)",
		"project", c.Project,
		"response", responseTokens,
		"current", state.CurrentTokens,
		"max", maxTokens,
		"total", state.TokenUsage.TotalTokens,
		"prompt", state.TokenUsage.TotalPromptTokens,
		"response_total", state.TokenUsage.TotalResponseTokens)

	// Save updated state
	if err := c.StateManager.Save(state); err != nil {
		telemetry.LogInfo("Warning: Failed to save state", "project", c.Project, "error", err)
	}
}

// SendWithRetry handles the common retry loop and telemetry for Send.
func (c *BaseClient) SendWithRetry(ctx context.Context, prompt string, sendOnce func(context.Context, string) (string, error)) (string, error) {
	telemetry.TrackAgentIteration(c.Project)
	start := time.Now()
	defer func() {
		telemetry.ObserveAgentLatency(c.Project, time.Since(start).Seconds())
	}()

	prompt, state, shouldUpdateState, err := c.PreparePrompt(prompt)
	if err != nil {
		return "", err
	}

	maxRetries := 3
	var lastErr error

	for i := 0; i <= maxRetries; i++ {
		if i > 0 {
			waitTime := c.BackoffFn(i)
			telemetry.LogInfo("Retrying agent call", "project", c.Project, "retry", i, "wait", waitTime, "error", lastErr)
			select {
			case <-time.After(waitTime):
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}

		result, err := sendOnce(ctx, prompt)
		if err == nil {
			if shouldUpdateState {
				c.UpdateStateWithResponse(state, result)
			}
			return result, nil
		}

		lastErr = err
	}

	return "", fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

// SendStreamWithRetry handles the common retry loop and telemetry for SendStream.
func (c *BaseClient) SendStreamWithRetry(ctx context.Context, prompt string, sendStreamOnce func(context.Context, string, func(string)) (string, error), onChunk func(string)) (string, error) {
	telemetry.TrackAgentIteration(c.Project)
	start := time.Now()
	defer func() {
		telemetry.ObserveAgentLatency(c.Project, time.Since(start).Seconds())
	}()

	prompt, state, shouldUpdateState, err := c.PreparePrompt(prompt)
	if err != nil {
		return "", err
	}

	maxRetries := 3
	var lastErr error

	for i := 0; i <= maxRetries; i++ {
		if i > 0 {
			waitTime := c.BackoffFn(i)
			telemetry.LogInfo("Retrying agent call", "project", c.Project, "retry", i, "wait", waitTime, "error", lastErr)
			select {
			case <-time.After(waitTime):
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}

		// When retrying stream, we need to handle onChunk carefully.
		// Usually we restart the stream.
		// Note: if sendStreamOnce partially writes to onChunk before failing,
		// the consumer might get duplicate chunks if we just retry.
		// However, in this implementation, we assume that if it fails, we restart from scratch.
		// The onChunk callback in the caller is responsible for handling partial updates if needed,
		// but typically for a TUI, rewriting is fine.

		result, err := sendStreamOnce(ctx, prompt, onChunk)
		if err == nil {
			if shouldUpdateState {
				c.UpdateStateWithResponse(state, result)
			}
			return result, nil
		}
		lastErr = err
	}

	return "", fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}
