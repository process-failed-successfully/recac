package agent

import (
	"context"
	"fmt"
	"recac/internal/telemetry"
	"time"
)

// AgentClient is a common interface for agent clients that support state management
type AgentClient interface {
	SendOnce(ctx context.Context, prompt string) (string, error)
	GetProject() string
	GetStateManager() *StateManager
	GetBackoffFn() func(int) time.Duration
	GetDefaultMaxTokens() int
}

// SendWithState handles the common logic for sending a prompt with state management, token tracking, and retries.
func SendWithState(ctx context.Context, client AgentClient, prompt string) (string, error) {
	project := client.GetProject()
	stateManager := client.GetStateManager()

	telemetry.TrackAgentIteration(project)
	start := time.Now()
	defer func() {
		telemetry.ObserveAgentLatency(project, time.Since(start).Seconds())
	}()

	// Load state and check token limits if state manager is configured
	var state State
	var shouldUpdateState bool
	if stateManager != nil {
		var err error
		state, err = stateManager.Load()
		if err != nil {
			return "", fmt.Errorf("failed to load state: %w", err)
		}
		shouldUpdateState = true

		// Check if prompt exceeds token limit
		promptTokens := EstimateTokenCount(prompt)
		maxTokens := state.MaxTokens
		if maxTokens == 0 {
			maxTokens = client.GetDefaultMaxTokens()
		}

		// Reserve some tokens for response (estimate 50% for response)
		availableTokens := maxTokens * 50 / 100
		if promptTokens > availableTokens {
			// Truncate the prompt
			telemetry.LogInfo("Prompt exceeds token limit, truncating...", "project", project, "actual", promptTokens, "available", availableTokens)
			prompt = TruncateToTokenLimit(prompt, availableTokens)
			promptTokens = EstimateTokenCount(prompt)
			state.TokenUsage.TruncationCount++
		}

		// Update current token count
		state.CurrentTokens = promptTokens
		state.TokenUsage.TotalPromptTokens += promptTokens
		telemetry.TrackTokenUsage(project, promptTokens)

		// Log token usage
		telemetry.LogDebug("Token usage (prompt)",
			"project", project,
			"prompt", promptTokens,
			"current", state.CurrentTokens,
			"max", maxTokens,
			"total_prompt", state.TokenUsage.TotalPromptTokens,
			"truncations", state.TokenUsage.TruncationCount)
	}

	maxRetries := 3
	var lastErr error
	backoffFn := client.GetBackoffFn()
	if backoffFn == nil {
		backoffFn = func(i int) time.Duration {
			return time.Duration(1<<uint(i-1)) * time.Second
		}
	}

	for i := 0; i <= maxRetries; i++ {
		if i > 0 {
			// Exponential backoff
			waitTime := backoffFn(i)
			telemetry.LogInfo("Retrying agent call", "project", project, "retry", i, "wait", waitTime, "error", lastErr)
			select {
			case <-time.After(waitTime):
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}

		result, err := client.SendOnce(ctx, prompt)
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
					maxTokens = client.GetDefaultMaxTokens()
				}
				telemetry.LogInfo("Token usage (response)",
					"project", project,
					"response", responseTokens,
					"current", state.CurrentTokens,
					"max", maxTokens,
					"total", state.TokenUsage.TotalTokens,
					"prompt", state.TokenUsage.TotalPromptTokens,
					"response_total", state.TokenUsage.TotalResponseTokens)

				// Save updated state
				if err := stateManager.Save(state); err != nil {
					telemetry.LogInfo("Warning: Failed to save state", "project", project, "error", err)
				}
			}
			return result, nil
		}

		lastErr = err
	}

	return "", fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}
