package agent

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
	return &OpenRouterClient{
		BaseClient: NewBaseClient(project, 128000), // Default generic limit
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
	return HTTPClientConfig{
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

// SendImage sends a prompt along with an image to OpenRouter
func (c *OpenRouterClient) SendImage(ctx context.Context, prompt string, imagePath string) (string, error) {
	// Read image file
	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		return "", fmt.Errorf("failed to read image file: %w", err)
	}

	// Detect MIME type based on extension
	ext := strings.ToLower(filepath.Ext(imagePath))
	var mimeType string
	switch ext {
	case ".png":
		mimeType = "image/png"
	case ".jpg", ".jpeg":
		mimeType = "image/jpeg"
	case ".webp":
		mimeType = "image/webp"
	case ".gif":
		mimeType = "image/gif"
	default:
		return "", fmt.Errorf("unsupported image format: %s", ext)
	}

	// Encode to base64
	base64Image := base64.StdEncoding.EncodeToString(imageData)
	dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Image)

	return SendImageOnce(ctx, c.getConfig(), prompt, dataURI)
}
