package agent

import (
	"context"
	"net/http"
	"time"
)

// OpenAIClient represents a client for the OpenAI API
type OpenAIClient struct {
	BaseClient
	apiKey     string
	model      string
	httpClient *http.Client
	apiURL     string
	// mockResponder is used for testing to bypass real API calls
	mockResponder func(string) (string, error)
}

// NewOpenAIClient creates a new OpenAI client
func NewOpenAIClient(apiKey, model, project string) *OpenAIClient {
	return &OpenAIClient{
		BaseClient: NewBaseClient(project, 128000), // Default to 128k for GPT-4
		apiKey:     apiKey,
		model:      model,
		httpClient: &http.Client{
			Timeout: 300 * time.Second,
		},
		apiURL: "https://api.openai.com/v1/chat/completions",
	}
}

// WithMockResponder sets a mock responder for testing
func (c *OpenAIClient) WithMockResponder(fn func(string) (string, error)) *OpenAIClient {
	c.mockResponder = fn
	return c
}

// WithStateManager sets the state manager for token tracking
func (c *OpenAIClient) WithStateManager(sm *StateManager) *OpenAIClient {
	c.StateManager = sm
	return c
}

func (c *OpenAIClient) getConfig() HTTPClientConfig {
	return HTTPClientConfig{
		BaseClient:    &c.BaseClient,
		APIKey:        c.apiKey,
		Model:         c.model,
		APIURL:        c.apiURL,
		HTTPClient:    c.httpClient,
		MockResponder: c.mockResponder,
	}
}

// Send sends a prompt to OpenAI and returns the generated text with retry logic.
// If stateManager is configured, it will track tokens and truncate if needed.
func (c *OpenAIClient) Send(ctx context.Context, prompt string) (string, error) {
	return c.SendWithRetry(ctx, prompt, c.sendOnce)
}

func (c *OpenAIClient) sendOnce(ctx context.Context, prompt string) (string, error) {
	return SendOnce(ctx, c.getConfig(), prompt)
}

// SendStream sends a prompt to OpenAI and streams the response
func (c *OpenAIClient) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return c.SendStreamWithRetry(ctx, prompt, func(ctx context.Context, p string, oc func(string)) (string, error) {
		return SendStreamOnce(ctx, c.getConfig(), p, oc)
	}, onChunk)
}
