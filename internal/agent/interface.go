package agent

import (
	"context"
	"fmt"
)

// Agent is the interface that all AI agents must implement
type Agent interface {
	// Send sends a prompt to the agent and returns the response
	Send(ctx context.Context, prompt string) (string, error)

	// SendStream sends a prompt to the agent and streams the response via the onChunk callback
	// Returns the full aggregated response at the end
	SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error)
}

// NewAgent is a factory function that returns an Agent based on the provider
// For Ollama, apiKey is used as baseURL (optional, defaults to http://localhost:11434)
func NewAgent(provider, apiKey, model, workDir string) (Agent, error) {
	switch provider {
	case "gemini":
		return NewGeminiClient(apiKey, model), nil
	case "gemini-cli":
		return NewGeminiCLIClient(apiKey, model, workDir), nil
	case "openai":
		return NewOpenAIClient(apiKey, model), nil
	case "ollama":
		return NewOllamaClient(apiKey, model), nil
	case "openrouter":
		return NewOpenRouterClient(apiKey, model), nil
	case "cursor-cli":
		return NewCursorCLIClient(apiKey, model), nil
	default:
		return nil, fmt.Errorf("unknown provider: %s", provider)
	}
}
