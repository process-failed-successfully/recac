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

	// Heuristics for E2E Tests (Smoke Test support)
	// 1. Planning Phase (TPM): Returns JSON ticket plan
	if strings.Contains(strings.ToLower(prompt), "technical program manager") {
		return `{
  "tickets": [
    {
      "id": "req-the-script-primes-py-is-implem",
      "title": "ID:[PRIMES] Implement Primes Script",
      "description": "Implement the primes.py script to calculate primes.",
      "type": "Task",
      "status": "Ready",
      "priority": "High"
    }
  ]
}`, nil
	}

	// 2. Coding Phase: Returns Bash script for implementation
	// We detect this if the prompt asks to implement code or contains typical coding agent instructions
	if strings.Contains(strings.ToLower(prompt), "coding agent") || strings.Contains(prompt, "Implement") {
		return "```bash\n" +
			"cat <<EOF > primes.py\n" +
			"import json\n" +
			"primes = []\n" +
			"for num in range(2, 10000):\n" +
			"    is_prime = True\n" +
			"    for i in range(2, int(num ** 0.5) + 1):\n" +
			"        if num % i == 0:\n" +
			"            is_prime = False\n" +
			"            break\n" +
			"    if is_prime:\n" +
			"        primes.append(num)\n" +
			"print(f\"Generated {len(primes)} primes\")\n" +
			"with open('primes.json', 'w') as f:\n" +
			"    json.dump({\"primes\": primes}, f)\n" +
			"EOF\n" +
			"\n" +
			"python3 primes.py\n" +
			"```", nil
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
