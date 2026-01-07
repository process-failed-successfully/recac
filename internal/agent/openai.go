package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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

// Send sends a prompt to OpenAI and returns the generated text with retry logic.
// If stateManager is configured, it will track tokens and truncate if needed.
func (c *OpenAIClient) Send(ctx context.Context, prompt string) (string, error) {
	return c.SendWithRetry(ctx, prompt, c.sendOnce)
}

func (c *OpenAIClient) sendOnce(ctx context.Context, prompt string) (string, error) {
	if c.mockResponder != nil {
		return c.mockResponder(prompt)
	}

	if c.apiKey == "" {
		return "", fmt.Errorf("API key is required")
	}

	requestBody := map[string]interface{}{
		"model": c.model,
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": prompt,
			},
		},
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("OpenAI API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no content in response")
	}

	return response.Choices[0].Message.Content, nil
}

// SendStream sends a prompt to OpenAI and streams the response
func (c *OpenAIClient) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return c.SendStreamWithRetry(ctx, prompt, func(ctx context.Context, p string, oc func(string)) (string, error) {
		// Prepare Request with the potentially truncated prompt 'p'
		requestBody := map[string]interface{}{
			"model":  c.model,
			"stream": true,
			"messages": []map[string]interface{}{
				{
					"role":    "user",
					"content": p,
				},
			},
		}
		return c.sendStreamOnce(ctx, p, requestBody, oc)
	}, onChunk)
}

func (c *OpenAIClient) sendStreamOnce(ctx context.Context, prompt string, requestBody map[string]interface{}, onChunk func(string)) (string, error) {
	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("OpenAI API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var fullResponse strings.Builder
	reader := bufio.NewReader(resp.Body)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", fmt.Errorf("error reading stream: %w", err)
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var streamResp struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}

		if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
			continue // Skip malformed lines
		}

		if len(streamResp.Choices) > 0 {
			content := streamResp.Choices[0].Delta.Content
			if content != "" {
				fullResponse.WriteString(content)
				if onChunk != nil {
					onChunk(content)
				}
			}
		}
	}

	return fullResponse.String(), nil
}
