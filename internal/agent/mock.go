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

	// TPM Agent: Ticket Generation
	// Checks for key phrases from tpm_agent.md prompt
	if strings.Contains(prompt, "Technical Program Manager") && (strings.Contains(prompt, "ID:[PRIMES]") || strings.Contains(prompt, "Prime Number Script")) {
		return `[
  {
    "title": "ID:[PRIMES] Prime Number Generator",
    "description": "Implement a Python script to generate prime numbers.",
    "type": "Task",
    "children": []
  }
]`, nil
	}

	// Coding Agent: Implementation
	// Checks for key phrases from coding_agent.md prompt
	if (strings.Contains(prompt, "CODING AGENT") || strings.Contains(prompt, "coding_agent")) &&
	   (strings.Contains(prompt, "ID:[PRIMES]") || strings.Contains(prompt, "prime") || strings.Contains(prompt, "Prime")) {
		return `
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n < 2: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = [n for n in range(10000) if is_prime(n)]

with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
EOF

python3 primes.py

git add primes.py primes.json
git commit -m "Implement primes"
git push origin HEAD

agent-bridge feature set $RECAC_PROJECT_ID --status done --passes true
agent-bridge signal COMPLETED true
`, nil
	}

	// Default fallback
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
