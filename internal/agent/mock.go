package agent

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// MockAgent is a simple mock agent for testing and mock mode
// It returns predefined responses without making actual API calls
type MockAgent struct {
	responsePrefix string
	forcedResponse string
}

// NewMockAgent creates a new mock agent
func NewMockAgent() *MockAgent {
	return &MockAgent{
		responsePrefix: "Mock agent response",
	}
}

// SetResponse forces a specific response from the agent
func (m *MockAgent) SetResponse(response string) {
	m.forcedResponse = response
}

// Send implements the Agent interface
// It returns a mock response that acknowledges the prompt
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// Heuristic: Check if this is a Planning Phase prompt (TPM Agent)
	// We check for keywords that appear in the TPM prompt or the expected output format.
	if contains(prompt, "Technical Program Manager") || contains(prompt, "ID:[PRIMES]") {
		// Return a predefined JSON response compatible with the CLI's expectations
		// Extract repo URL from prompt if possible to make it more realistic
		repoURL := "https://github.com/example/repo"
		if matches := regexp.MustCompile(`(?i)Repo: (https?://\S+)`).FindStringSubmatch(prompt); len(matches) > 1 {
			repoURL = matches[1]
		}

		return fmt.Sprintf(`[
  {
    "title": "ID:[PRIMES] Implement Primes Service",
    "description": "Implement a service that calculates prime numbers. Repo: %s",
    "type": "Epic",
    "acceptance_criteria": [
      "Service calculates primes correctly",
      "API returns JSON"
    ],
    "children": [
      {
        "title": "ID:[PRIMES-API] Implement API",
        "description": "Implement the HTTP API for the primes service. Repo: %s",
        "type": "Story",
        "acceptance_criteria": [
          "GET /primes/{n} returns prime check",
          "Returns 200 OK"
        ]
      }
    ]
  }
]`, repoURL, repoURL), nil
	}

	// Heuristic: Check if this is an Execution Phase prompt (Agent Role)
	// If features are pending, we update them. If done, we signal completion.
	// Use regex for flexible whitespace matching on json status
	if strings.Contains(prompt, `"status": "pending"`) || regexp.MustCompile(`"status":\s*"pending"`).MatchString(prompt) {
		return `Here is a plan to implement the pending features.

` + "```bash" + `
#!/bin/bash
if [ -f feature_list.json ]; then
  sed -i 's/"status": "pending"/"status": "done"/g' feature_list.json
  echo "Success: Mock command executed"
  # Also signal completion to DB to ensure the loop exits
  agent-bridge signal COMPLETED true || echo "agent-bridge failed, continuing"
else
  echo "Error: feature_list.json not found"
fi
` + "```" + `
`, nil
	}

	// Check for done status with flexible whitespace or explicit "All features" signal
	if strings.Contains(prompt, "All features") || regexp.MustCompile(`"status":\s*"done"`).MatchString(prompt) {
		return "Task completed. All features implemented.", nil
	}

	// Fallback for Agent Execution loop to prevent infinite no-op loops in tests:
	// If the prompt looks like it contains a feature list (has "project_name" or "features") but didn't match above,
	// assume it's an ambiguous state or "done" state that wasn't caught. Return "Task completed" to exit safely.
	if strings.Contains(prompt, "project_name") && strings.Contains(prompt, "features") {
		return "Task completed. No pending features detected.", nil
	}

	// Return a mock response that shows the agent received the prompt
	// This allows the session to run without requiring real API keys
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100))
	return response, nil
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// SendStream implements the Agent interface
func (m *MockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	resp, err := m.Send(ctx, prompt)
	if err == nil && onChunk != nil {
		onChunk(resp)
	}
	return resp, err
}

// truncateString truncates a string to a maximum length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
