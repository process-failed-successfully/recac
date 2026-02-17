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

	// Heuristic: If prompt asks for JSON tickets (TPM or ID:[PRIMES]), return JSON
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "ID:[PRIMES]") {
		// Extract repo URL if present to make ticket valid for JiraPoller
		repoURL := "https://github.com/example/repo"
		re := regexp.MustCompile(`(?i)Repo: (https?://\S+)`)
		matches := re.FindStringSubmatch(prompt)
		if len(matches) > 1 {
			repoURL = matches[1]
		}

		return fmt.Sprintf(`[
  {
    "title": "Implement Prime Number Generator",
    "description": "Create a Python script that generates prime numbers.\n\nREQUIRED FEATURES:\n- Function to check if a number is prime\n- Main loop to print primes\n\nRepo: %s",
    "type": "Task",
    "labels": ["recac-agent"]
  }
]`, repoURL), nil
	}

	// Heuristic: If prompt looks like a coding task (but not the TPM prompt), return a coding response
	// We want to avoid matching the TPM prompt which also contains "Python" etc.
	if (strings.Contains(prompt, "Python") || strings.Contains(prompt, "Go") || strings.Contains(prompt, "code")) && !strings.Contains(prompt, "Technical Program Manager") {
		return `I will implement the requested changes.

$$$
#!/bin/bash
# Implementation
echo "Implementing feature..."
# Create a dummy file to satisfy verification
echo "def is_prime(n): return n > 1" > primes.py

# Mark features as done
echo "Marking features as done..."
ids=$(agent-bridge feature list | jq -r '.features[].id')
for id in $ids; do
  agent-bridge feature set "$id" --status done --passes true
done
$$$

Task completed. Tests passed.
`, nil
	}

	// Heuristic: QA Agent
	if strings.Contains(prompt, "QA Agent") {
		return "QA_PASSED", nil
	}

	// Heuristic: Manager Agent
	if strings.Contains(prompt, "Manager Agent") {
		return "PROJECT_SIGNED_OFF", nil
	}

	// Return a mock response that shows the agent received the prompt
	// This allows the session to run without requiring real API keys
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100))
	return response, nil
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
