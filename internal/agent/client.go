package agent

import (
	"context"
	"fmt"
	"recac/internal/telemetry"
	"time"
)

// sendRequester defines the interface for provider-specific logic
// that the BaseClient will use to send requests.
type sendRequester interface {
	sendOnce(ctx context.Context, prompt string) (string, error)
	getDefaultMaxTokens() int
}

// BaseClient encapsulates common logic for agent clients, such as state management,
// token tracking, and retry mechanisms.
type BaseClient struct {
	project       string
	stateManager  *StateManager
	backoffFn     func(int) time.Duration
	sendRequester sendRequester
}

// NewBaseClient creates a new BaseClient.
func NewBaseClient(project string, stateManager *StateManager, requester sendRequester) *BaseClient {
	return &BaseClient{
		project:      project,
		stateManager: stateManager,
		sendRequester: requester,
		backoffFn: func(retry int) time.Duration {
			return time.Duration(1<<uint(retry-1)) * time.Second
		},
	}
}

// Send handles the common logic for sending a prompt, including state management,
// token truncation, and retries. It delegates the actual API call to the
// embedded sendRequester.
func (c *BaseClient) Send(ctx context.Context, prompt string) (string, error) {
	telemetry.TrackAgentIteration(c.project)
	start := time.Now()
	defer func() {
		telemetry.ObserveAgentLatency(c.project, time.Since(start).Seconds())
	}()

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
			maxTokens = c.sendRequester.getDefaultMaxTokens()
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
			waitTime := c.backoffFn(i)
			telemetry.LogInfo("Retrying agent call", "project", c.project, "retry", i, "wait", waitTime, "error", lastErr)
			select {
			case <-time.After(waitTime):
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}

		result, err := c.sendRequester.sendOnce(ctx, prompt)
		if err == nil {
			if shouldUpdateState {
				responseTokens := EstimateTokenCount(result)
				state.TokenUsage.TotalResponseTokens += responseTokens
				state.TokenUsage.TotalTokens = state.TokenUsage.TotalPromptTokens + state.TokenUsage.TotalResponseTokens
				state.CurrentTokens += responseTokens

				if state.Metadata == nil {
					state.Metadata = make(map[string]interface{})
				}

				currentIteration, _ := state.Metadata["iteration"].(float64)
				state.Metadata["iteration"] = currentIteration + 1

				maxTokens := state.MaxTokens
				if maxTokens == 0 {
					maxTokens = c.sendRequester.getDefaultMaxTokens()
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
					fmt.Printf("Warning: Failed to save state: %v\n", err)
				}
			}
			return result, nil
		}
		lastErr = err
	}
	return "", fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}
