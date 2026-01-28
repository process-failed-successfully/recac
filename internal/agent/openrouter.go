package agent

import (
	"context"
	"net/http"
	"os"
	"time"
)

// OpenRouterClient implements the Agent interface for OpenRouter
type OpenRouterClient struct {
	BaseClient
	apiKey     string
	model      string
	httpClient *http.Client
	apiURL     string
	// mockResponder is used for testing to bypass real API calls
	mockResponder func(string) (string, error)
}

// NewOpenRouterClient creates a new OpenRouter client
func NewOpenRouterClient(apiKey, model, project string) *OpenRouterClient {
	// Default generic limit
	maxTokens := 128000

	// Reduce context window in CI to avoid rate limits/credit exhaustion
	// The smoke test logs showed HTTP 402 errors (insufficient credits) because it requested too many tokens.
	if os.Getenv("CI") == "true" || os.Getenv("RECAC_CI_MODE") == "true" {
		maxTokens = 4096
	}

	return &OpenRouterClient{
		BaseClient: NewBaseClient(project, maxTokens),
		apiKey:     apiKey,
		model:      model,
		httpClient: &http.Client{
			Timeout: 600 * time.Second, // OpenRouter can be slower depending on the underlying model
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
	c.StateManager = sm
	return c
}

func (c *OpenRouterClient) getConfig() HTTPClientConfig {
	cfg := HTTPClientConfig{
		BaseClient:    &c.BaseClient,
		APIKey:        c.apiKey,
		Model:         c.model,
		APIURL:        c.apiURL,
		HTTPClient:    c.httpClient,
		MockResponder: c.mockResponder,
		Headers: map[string]string{
			"HTTP-Referer": "https://github.com/process-failed-successfully/recac",
			"X-Title":      "Process Failed Successfully",
		},
	}

	// Only set MaxTokens if it's a reduced value (indicating CI or constrained environment)
	// to avoid overriding model defaults with a large number like 128000.
	if c.BaseClient.DefaultMaxTokens > 0 && c.BaseClient.DefaultMaxTokens <= 8192 {
		cfg.MaxTokens = c.BaseClient.DefaultMaxTokens
	}

	return cfg
}

// Send sends a prompt to OpenRouter and returns the generated text
func (c *OpenRouterClient) Send(ctx context.Context, prompt string) (string, error) {
	return c.SendWithRetry(ctx, prompt, c.sendOnce)
}

func (c *OpenRouterClient) sendOnce(ctx context.Context, prompt string) (string, error) {
	return SendOnce(ctx, c.getConfig(), prompt)
}

// SendStream sends a prompt to OpenRouter and streams the response
func (c *OpenRouterClient) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return c.SendStreamWithRetry(ctx, prompt, func(ctx context.Context, p string, oc func(string)) (string, error) {
		return SendStreamOnce(ctx, c.getConfig(), p, oc)
	}, onChunk)
}
