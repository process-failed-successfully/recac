package agent

import (
	"context"
	"fmt"
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

	// Heuristic: Check if prompt asks for JSON ticket plan (TPM Role)
	if isTicketGenerationPrompt(prompt) {
		return generateMockTicketPlan(), nil
	}

	// Heuristic: Check if prompt asks for python implementation (Coding Role)
	if isPythonImplementationPrompt(prompt) {
		return generateMockPythonImplementation(), nil
	}

	// Return a mock response that shows the agent received the prompt
	// This allows the session to run without requiring real API keys
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100))
	return response, nil
}

// isTicketGenerationPrompt checks if the prompt is asking for a ticket plan
func isTicketGenerationPrompt(prompt string) bool {
	// Check for keywords indicating ticket generation request
	// This should align with the prompts used in the E2E scenario
	return containsAny(prompt, "Technical Program Manager", "TPM", "ticket plan", "user stories", "epics") &&
		containsAny(prompt, "JSON", "json")
}

// isPythonImplementationPrompt checks if the prompt is asking for python implementation
func isPythonImplementationPrompt(prompt string) bool {
	return containsAny(prompt, "python", "script", "primes") && containsAny(prompt, "implement", "code")
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if hasSubstring(s, sub) {
			return true
		}
	}
	return false
}

func hasSubstring(s, sub string) bool {
	// Case-insensitive check
	return len(s) >= len(sub) && (s == sub || search(s, sub))
}

// Simple search helper (can be replaced with strings.Contains(strings.ToLower(s), strings.ToLower(sub)))
func search(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}

func generateMockTicketPlan() string {
	return `[
  {
    "title": "Implement Prime Number Script",
    "description": "Create a python script primes.py that calculates and prints the first 10 prime numbers.",
    "type": "Task",
    "labels": ["backend", "python"],
    "dependencies": []
  }
]`
}

func generateMockPythonImplementation() string {
	return "```python\ndef is_prime(n):\n    if n <= 1:\n        return False\n    for i in range(2, int(n**0.5) + 1):\n        if n % i == 0:\n            return False\n    return True\n\nprimes = []\nnum = 2\nwhile len(primes) < 10:\n    if is_prime(num):\n        primes.append(num)\n    num += 1\n\nprint(primes)\n```"
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
