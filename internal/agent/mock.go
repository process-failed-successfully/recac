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

	// Heuristics for E2E Tests

	// 1. Prime Number Scenario
	if strings.Contains(prompt, "primes.py") && strings.Contains(prompt, "10,000") {
		return m.getPrimePythonResponse(), nil
	}

	// 2. Feature List Generation (if applicable)
	if strings.Contains(prompt, "Technical Program Manager") && strings.Contains(prompt, "feature_list.json") {
		return m.getFeatureListResponse(), nil
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

// getPrimePythonResponse returns a valid python script for the primes scenario
func (m *MockAgent) getPrimePythonResponse() string {
	return `Here is the solution to generate primes < 10,000.

` + "```bash" + `
cat << 'EOF' > primes.py
import json

def get_primes(n):
    primes = []
    for num in range(2, n):
        is_prime = True
        for i in range(2, int(num**0.5) + 1):
            if num % i == 0:
                is_prime = False
                break
        if is_prime:
            primes.append(num)
    return primes

primes = get_primes(10000)
with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
print(f"Generated {len(primes)} primes.")
EOF

# Run the script
python3 primes.py

# Commit
git add primes.py primes.json
git commit -m "Add primes generation script" || echo "Nothing to commit"
` + "```" + `
`
}

func (m *MockAgent) getFeatureListResponse() string {
	return `
` + "```bash" + `
cat << 'EOF' > feature_list.json
{
  "project_name": "mock-project",
  "features": [
    {
      "id": "PRIMES",
      "description": "Prime Number Script",
      "priority": "High",
      "status": "Pending"
    }
  ]
}
EOF
` + "```" + `
`
}
