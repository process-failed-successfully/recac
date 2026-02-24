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

	var ag Agent

	switch provider {
	case "gemini":
		ag = NewGeminiClient(apiKey, model, project)
	case "gemini-cli":
		ag = NewGeminiCLIClient(apiKey, model, workDir, project)
	case "openai":
		ag = NewOpenAIClient(apiKey, model, project)
	case "ollama":
		ag = NewOllamaClient(apiKey, model, project)
	case "openrouter":
		ag = NewOpenRouterClient(apiKey, model, project)
	case "cursor-cli":
		ag = NewCursorCLIClient(apiKey, model, project)
	case "opencode", "opencode-cli":
		ag = NewOpenCodeCLIClient(apiKey, model, workDir, project)
	case "mock":
		ag = NewMockAgent()
	default:
		return nil, fmt.Errorf("unknown provider: %s", provider)
	}

	// Wrap with DirectiveAgent to enforce project-wide instructions
	return NewDirectiveAgent(ag, workDir), nil
}
