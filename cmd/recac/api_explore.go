package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"recac/internal/utils"

	"github.com/spf13/viper"
)

// DiscoveredEndpoint represents a discovered API endpoint.
type DiscoveredEndpoint struct {
	Method      string            `json:"method"`
	Path        string            `json:"path"`
	Description string            `json:"description"`
	Headers     map[string]string `json:"headers,omitempty"`
	Body        string            `json:"body_sample,omitempty"` // Sample body as JSON string
}

// scanEndpointsWithAI scans the codebase using AI to find API endpoints.
func scanEndpointsWithAI(ctx context.Context, root string) ([]DiscoveredEndpoint, error) {
	// 1. Generate Context
	opts := ContextOptions{
		Roots:     []string{root},
		MaxSize:   100 * 1024, // 100KB per file
		NoContent: false,
		Tree:      true,
	}

	// We need access to GenerateCodebaseContext which is in main package
	// Since we are in main package, we can call it directly.
	codebaseContext, err := GenerateCodebaseContext(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to generate codebase context: %w", err)
	}

	// 2. Prepare Agent
	cwd, _ := os.Getwd()
	provider := viper.GetString("provider")
	model := viper.GetString("model")

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-api-explore")
	if err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	// 3. Prompt
	prompt := fmt.Sprintf(`Analyze the codebase context provided below and identify all HTTP API endpoints.
Look for route definitions in any framework (Gin, Echo, Chi, Fiber, net/http, Express, Flask, etc.).

Return a JSON array of objects with the following structure:
[
  {
    "method": "GET|POST|PUT|DELETE|PATCH",
    "path": "/api/v1/resource/:id",
    "description": "Short description of what this endpoint does",
    "headers": {"Content-Type": "application/json"},
    "body_sample": "{\"key\": \"value\"}" // JSON string of a sample request body if applicable (POST/PUT)
  }
]

IMPORTANT:
- Return ONLY the JSON array.
- Do not include markdown formatting like '''json ... '''.
- If body_sample is not applicable, omit it or set to null.
- Infer the description from comments or code logic.

CODEBASE CONTEXT:
%s`, codebaseContext)

	// 4. Send
	// We use Send, not Stream, because we need the full JSON to parse.
	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("agent failed: %w", err)
	}

	// 5. Parse
	jsonStr := utils.CleanJSONBlock(resp)
	var endpoints []DiscoveredEndpoint
	if err := json.Unmarshal([]byte(jsonStr), &endpoints); err != nil {
		// Fallback: try to fix common JSON errors or return raw error
		return nil, fmt.Errorf("failed to parse AI response as JSON: %w\nResponse was:\n%s", err, resp)
	}

	return endpoints, nil
}

// executeApiRequest executes an HTTP request for the given endpoint configuration.
func executeApiRequest(method, url string, headers map[string]string, body string) (string, int, time.Duration, error) {
	var bodyReader io.Reader
	if body != "" {
		bodyReader = bytes.NewBufferString(body)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return "", 0, 0, err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	start := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(start)
	if err != nil {
		return "", 0, duration, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", resp.StatusCode, duration, fmt.Errorf("failed to read response body: %w", err)
	}

	// Format JSON response if possible
	var prettyJSON bytes.Buffer
	if json.Indent(&prettyJSON, respBody, "", "  ") == nil {
		return prettyJSON.String(), resp.StatusCode, duration, nil
	}

	return string(respBody), resp.StatusCode, duration, nil
}
