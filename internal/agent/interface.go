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
func NewAgent(provider, apiKey, model, workDir, project string) (Agent, error) {
	// Default to "unknown" if project is empty
	if project == "" {
		project = "unknown"
	}

	switch provider {
	case "gemini":
		return NewGeminiClient(apiKey, model, project), nil
	case "gemini-cli":
		return NewGeminiCLIClient(apiKey, model, workDir, project), nil
	case "openai":
		return NewOpenAIClient(apiKey, model, project), nil
	case "ollama":
		return NewOllamaClient(apiKey, model, project), nil
	case "openrouter":
		return NewOpenRouterClient(apiKey, model, project), nil
	case "cursor-cli":
		return NewCursorCLIClient(apiKey, model, project), nil
	default:
		return nil, fmt.Errorf("unknown provider: %s", provider)
	}
}
