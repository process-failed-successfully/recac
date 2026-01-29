package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
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
}

// SendOnce performs a single non-streaming request
func SendOnce(ctx context.Context, cfg HTTPClientConfig, prompt string) (string, error) {
	if cfg.MockResponder != nil {
		return cfg.MockResponder(prompt)
	}

	if cfg.APIKey == "" {
		return "", fmt.Errorf("API key is required")
	}

	requestBody := map[string]interface{}{
		"model": cfg.Model,
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
		if resp.StatusCode == http.StatusTooManyRequests {
			return "", checkRateLimit(resp)
		}
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
	requestBody := map[string]interface{}{
		"model":  cfg.Model,
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
		if resp.StatusCode == http.StatusTooManyRequests {
			return "", checkRateLimit(resp)
		}
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

func checkRateLimit(resp *http.Response) error {
	bodyBytes, _ := io.ReadAll(resp.Body)
	msg := fmt.Sprintf("API returned status %d: %s", resp.StatusCode, string(bodyBytes))

	var resetTime time.Time
	var retryAfter time.Duration

	// Check X-RateLimit-Reset
	if resetStr := resp.Header.Get("X-RateLimit-Reset"); resetStr != "" {
		// Try parsing as float or int (millis or seconds)
		if resetMillis, err := strconv.ParseInt(resetStr, 10, 64); err == nil {
			// Check if it looks like millis (very large number) or seconds
			// 2026 in seconds is ~1.7e9, in millis ~1.7e12
			if resetMillis > 100000000000 { // Likely millis
				resetTime = time.UnixMilli(resetMillis)
			} else {
				resetTime = time.Unix(resetMillis, 0)
			}
		} else if resetFloat, err := strconv.ParseFloat(resetStr, 64); err == nil {
			// Some APIs return float seconds
			sec := int64(resetFloat)
			nsec := int64((resetFloat - float64(sec)) * 1e9)
			resetTime = time.Unix(sec, nsec)
		}
	}

	// Check Retry-After
	if retryStr := resp.Header.Get("Retry-After"); retryStr != "" {
		if seconds, err := strconv.Atoi(retryStr); err == nil {
			retryAfter = time.Duration(seconds) * time.Second
		} else if date, err := http.ParseTime(retryStr); err == nil {
			resetTime = date
		}
	}

	return RateLimitError{
		Message:    msg,
		ResetTime:  resetTime,
		RetryAfter: retryAfter,
	}
}
