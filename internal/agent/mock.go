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

	lowerPrompt := strings.ToLower(prompt)

	// Heuristic: TPM/Architect Phase
	// Check for "json" AND ("technical program manager" OR "architect")
	// The prompt usually asks the TPM to create tickets in JSON format.
	if strings.Contains(lowerPrompt, "json") && (strings.Contains(lowerPrompt, "technical program manager") || strings.Contains(lowerPrompt, "architect")) {
		// Return a JSON list of tickets suitable for the "prime-python" scenario.
		// We include the repo URL as per requirements to avoid git errors.
		return `[
  {
    "id": "PRIMES",
    "title": "ID:[PRIMES] Generate Primes",
    "description": "Generate prime numbers using Python. Repo: https://github.com/process-failed-successfully/recac-jira-e2e",
    "type": "Story",
    "status": "Todo",
    "story_points": 3,
    "dependencies": [],
    "children": []
  }
]`, nil
	}

	// Heuristic: Coding Phase (Primes)
	// Triggers on:
	// 1. "id:[primes]" (from ticket summary)
	// 2. "generate primes" (from description)
	// 3. "primes.json" (from task requirements)
	// 4. "prime" AND "python" (fallback)
	// 5. "primes" (relaxed fallback for E2E robustness)
	// AND NOT "technical program manager" (to avoid TPM phase overlap)
	// AND NOT "architect" (to avoid Architect phase overlap)
	if (strings.Contains(lowerPrompt, "id:[primes]") ||
		strings.Contains(lowerPrompt, "generate primes") ||
		strings.Contains(lowerPrompt, "primes.json") ||
		(strings.Contains(lowerPrompt, "prime") && strings.Contains(lowerPrompt, "python")) ||
		strings.Contains(lowerPrompt, "primes")) &&
		!strings.Contains(lowerPrompt, "technical program manager") &&
		!strings.Contains(lowerPrompt, "architect") {
		return "```python\nimport json\n\ndef is_prime(n):\n    if n <= 1: return False\n    for i in range(2, int(n**0.5) + 1):\n        if n % i == 0: return False\n    return True\n\nprimes = [x for x in range(1, 101) if is_prime(x)]\nwith open('primes.json', 'w') as f:\n    json.dump({\"primes\": primes}, f)\nprint(\"Generated primes.json\")\n```", nil
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
