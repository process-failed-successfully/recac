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
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// Heuristic for E2E Prime Python Scenario
	if strings.Contains(prompt, "primes.py") || strings.Contains(prompt, "[PRIMES]") {
		return m.generatePrimesResponse(), nil
	}

	// Return a generic mock response
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

func (m *MockAgent) generatePrimesResponse() string {
	return `
Here is the implementation for the prime number script.

` + "```bash" + `
# Configure git identity for CI
git config user.email "agent@recac.com"
git config user.name "Recac Agent"

# Create the python script
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n < 2: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = [i for i in range(10000) if is_prime(i)]
with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
EOF

# Run the script to generate the output file
python3 primes.py

# Commit and push
git add primes.py primes.json
git commit -m "Implement primes.py and generate primes.json"
git push origin HEAD
` + "```" + `
`
}

// truncateString truncates a string to a maximum length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
