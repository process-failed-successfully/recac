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
	lowerPrompt := strings.ToLower(prompt)

	// 1. Technical Program Manager / Planning
	if strings.Contains(lowerPrompt, "technical program manager") || strings.Contains(lowerPrompt, "tpm") {
		return `{
  "tickets": [
    {
      "title": "Implement Prime Number Generator",
      "description": "Write a python script that calculates the first 100 prime numbers and writes them to primes.json",
      "type": "task",
      "priority": "high"
    }
  ]
}`, nil
	}

	// 2. QA Agent / Manager Review
	if strings.Contains(lowerPrompt, "qa agent") || strings.Contains(lowerPrompt, "manager review") || strings.Contains(lowerPrompt, "project manager") {
		return "Code looks good. All tests passed. Proceeding to sign-off.", nil
	}

	// 3. Coding Agent - Primes Task
	if strings.Contains(prompt, "ID:[PRIMES]") || strings.Contains(prompt, "primes.py") || strings.Contains(prompt, "1229") {
		return `I will implement the prime number generator script as requested.

` + "```bash" + `
#!/bin/bash
# Create primes.py
cat <<EOF > primes.py
import json

def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

primes = []
num = 2
while len(primes) < 100:
    if is_prime(num):
        primes.append(num)
    num += 1

with open("primes.json", "w") as f:
    json.dump(primes, f)

print(f"Generated {len(primes)} primes to primes.json")
EOF

# Execute the script to generate artifacts
python3 primes.py

# Add and commit changes (handling idempotency)
git add primes.py primes.json
git commit -m "Implement prime number generator" || echo "nothing to commit"

# Mark features as completed
# Note: In a real scenario, these IDs would be dynamically extracted, but for this mock we hardcode common ones
agent-bridge feature set req-script-runs-without-errors completed || true
agent-bridge feature set req-calculates-first-100-primes-co completed || true
agent-bridge feature set req-outputs-results-to-primes-json completed || true

# Signal completion of the project
agent-bridge signal PROJECT_SIGNED_OFF true --privileged
` + "```" + `

I have implemented the script, generated the output, committed the changes, and signaled completion.
`, nil
	}

	// Fallback response
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
