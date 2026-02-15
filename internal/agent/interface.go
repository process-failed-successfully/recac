package agent

import (
	"context"
	"fmt"
	"strings"
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

	// Correct model name for OpenRouter if needed
	if provider == "openrouter" && !strings.Contains(model, "/") {
		originalModel := model
		if strings.HasPrefix(model, "gemini-") {
			model = "google/" + model
		} else if strings.HasPrefix(model, "gpt-") {
			model = "openai/" + model
		} else if strings.HasPrefix(model, "claude-") {
			model = "anthropic/" + model
		} else if strings.HasPrefix(model, "llama-") {
			model = "meta-llama/" + model
		} else if strings.HasPrefix(model, "mistral-") || strings.HasPrefix(model, "mixtral-") {
			model = "mistralai/" + model
		}

		if model != originalModel {
			fmt.Printf("Agent Factory: Corrected OpenRouter model from %s to %s\n", originalModel, model)
		}
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
	case "opencode", "opencode-cli":
		return NewOpenCodeCLIClient(apiKey, model, workDir, project), nil
	default:
		return nil, fmt.Errorf("unknown provider: %s", provider)
	}
}
