package agent

import (
	"context"
	"fmt"
)

// Agent is the interface that all AI agents must implement
type Agent interface {
	// Send sends a prompt to the agent and returns the response
	Send(ctx context.Context, prompt string) (string, error)
}

// NewAgent is a factory function that returns an Agent based on the provider
func NewAgent(provider, apiKey, model string) (Agent, error) {
	switch provider {
	case "gemini":
		return NewGeminiClient(apiKey, model), nil
	case "openai":
		return NewOpenAIClient(apiKey, model), nil
	default:
		return nil, fmt.Errorf("unknown provider: %s", provider)
	}
}
