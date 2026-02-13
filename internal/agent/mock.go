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

	// Heuristics for E2E scenarios

	// 1. Prime-Python Planning Phase (Ticket Generation)
	// Check if prompt is asking to generate tickets for [PRIMES] spec
	if strings.Contains(prompt, "ID:[PRIMES]") && strings.Contains(prompt, "Ticket") {
		// Extract repo URL if present
		repo := "https://github.com/process-failed-successfully/recac-jira-e2e"
		if strings.Contains(prompt, "Repo: ") {
			parts := strings.Split(prompt, "Repo: ")
			if len(parts) > 1 {
				repo = strings.TrimSpace(strings.Split(parts[1], "\n")[0])
			}
		}

		return fmt.Sprintf(`[
  {
    "title": "ID:[PRIMES] Create Prime Number Script",
    "description": "Implement primes.py to calculate primes < 10000 and output to primes.json.\nRepo: %s",
    "type": "Task",
    "children": []
  }
]`, repo), nil
	}

	// 2. Prime-Python Execution Phase
	// Check if prompt is asking to implement primes.py
	if (strings.Contains(strings.ToLower(prompt), "primes.py") || strings.Contains(strings.ToLower(prompt), "prime number")) &&
	   (strings.Contains(prompt, "write") || strings.Contains(prompt, "create") || strings.Contains(prompt, "implement")) {

		return `I will create the 'primes.py' script as requested and commit it.

` + "```bash" + `
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n <= 1: return False
    if n <= 3: return True
    if n % 2 == 0 or n % 3 == 0: return False
    i = 5
    while i * i <= n:
        if n % i == 0 or n % (i + 2) == 0: return False
        i += 6
    return True

primes = [i for i in range(10000) if is_prime(i)]
with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
EOF

python3 primes.py
git add primes.py primes.json
git commit -m "Generate primes"
` + "```" + `
`, nil
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
