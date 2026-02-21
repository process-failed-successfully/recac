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
)

// HTTPClientConfig defines the configuration for the shared HTTP client logic
type HTTPClientConfig struct {
	BaseClient    *BaseClient
	APIKey        string
	Model         string
	APIURL        string
	HTTPClient    *http.Client
	MockResponder func(string) (string, error)
	Headers       map[string]string
	DropModelPrefix bool
}

// SendOnce performs a single non-streaming request
func SendOnce(ctx context.Context, cfg HTTPClientConfig, prompt string) (string, error) {
	if cfg.MockResponder != nil {
		return cfg.MockResponder(prompt)
	}

	if cfg.APIKey == "" {
		return "", fmt.Errorf("API key is required")
	}

	modelID := cfg.Model
	if cfg.DropModelPrefix {
		// Strip common prefixes like "openrouter/" or others if needed
		// For now, OpenRouter seems to expect "openai/gpt-4" but not "openrouter/openai/gpt-4"
		// The issue in CI was `openrouter/google/gemini...`? No, it was just `google/gemini...` passed AS model.
		// Wait, the CI failure said: "google/gemini-2.0-flash-lite-preview-02-05:free is not a valid model ID"
		// This suggests OpenRouter expects us to use it, but maybe we ARE sending it correctly?
		// Actually, if we pass `openrouter/aurora-alpha`, the code might be stripping it?
		// No, `DropModelPrefix` wasn't there.
		// Let's implement the logic: If DropModelPrefix is true, and model starts with "openrouter/", strip it.
		if strings.HasPrefix(modelID, "openrouter/") {
			modelID = strings.TrimPrefix(modelID, "openrouter/")
		}
	}

	requestBody := map[string]interface{}{
		"model": modelID,
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

	req, err := http.NewRequestWithContext(ctx, "POST", cfg.APIURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	// Add custom headers
	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}

	resp, err := cfg.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(bodyBytes))
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

// SendStreamOnce performs a single streaming request
func SendStreamOnce(ctx context.Context, cfg HTTPClientConfig, prompt string, onChunk func(string)) (string, error) {
	modelID := cfg.Model
	if cfg.DropModelPrefix {
		if strings.HasPrefix(modelID, "openrouter/") {
			modelID = strings.TrimPrefix(modelID, "openrouter/")
		}
	}

	requestBody := map[string]interface{}{
		"model":  modelID,
		"stream": true,
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

	req, err := http.NewRequestWithContext(ctx, "POST", cfg.APIURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	// Add custom headers
	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}

	resp, err := cfg.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(bodyBytes))
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
