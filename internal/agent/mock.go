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

	// 1. Initializer Role: Detect request for feature list
	if strings.Contains(prompt, "feature_list.json") || strings.Contains(prompt, "Initializer") {
		return `I will initialize the project features.
` + "```bash" + `
set -e
cat <<EOF > /tmp/features.json
[
  {
    "id": "req-prime-1",
    "description": "Must correctly identify prime numbers",
    "type": "functional"
  },
  {
    "id": "req-prime-2",
    "description": "Must print primes up to 20",
    "type": "functional"
  }
]
EOF
agent-bridge feature import /tmp/features.json
` + "```", nil
	}

	// 2. TPM Role: If prompt asks for JSON ticket plan
	if strings.Contains(prompt, "Technical Program Manager") && strings.Contains(prompt, "JSON") {
		// TPM usually returns raw JSON
		return `{
  "id": "PRIMES",
  "project_name": "prime-python",
  "tickets": [
    {
      "title": "Implement Prime Number Generator",
      "description": "Create a Python script to generate prime numbers.",
      "type": "Task",
      "status": "Todo",
      "id": "MFLP-4985",
      "dependencies": []
    }
  ]
}`, nil
	}

	// 3. Agent Role (Implementation): Detect prime number task
	// Matches "Implement Prime Number Generator" or "primes.py"
	if strings.Contains(prompt, "Prime Number Generator") || strings.Contains(prompt, "primes.py") {
		return `I will implement the prime number generator.
` + "```bash" + `
set -e
cat <<EOF > primes.py
import json

def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = [x for x in range(2, 21) if is_prime(x)]
print(f"Primes: {primes}")

with open('primes.json', 'w') as f:
    json.dump(primes, f)
EOF

# Verify it works
python3 primes.py

# Commit
git add primes.py primes.json || true
git commit -m "Add prime number generator" || echo "Commit failed or nothing to commit"
# Push skipped in mock mode to avoid auth errors
echo "Push skipped"
` + "```", nil
	}

	// 4. QA / Manager Roles: Signal completion
	if strings.Contains(prompt, "QA AGENT") || strings.Contains(prompt, "PROJECT MANAGER") {
		return `Tests passed. Signing off.
` + "```bash" + `
agent-bridge signal set QA_PASSED true
agent-bridge signal set PROJECT_SIGNED_OFF true
` + "```", nil
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
