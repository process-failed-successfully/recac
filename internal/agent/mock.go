package agent

import (
	"context"
	"fmt"
	"strings"
)

// MockAgent is a simple mock agent for testing and mock mode
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

func (m *MockAgent) SetResponse(response string) {
	m.forcedResponse = response
}

func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// Heuristics for E2E tests

	// 1. Initializer / Planner (Often asks for a plan or feature list)
	// The real agent usually returns a JSON object with features.
	// For the smoke test, we might not hit this if we skip planning, but if we do:
	if strings.Contains(prompt, "feature_list.json") || strings.Contains(prompt, "Technical Program Manager") {
		// Return a list of tickets (array), not a feature object, to match CLI expectations
		return `
[
  {
    "id": "req-primes-py-exists",
    "name": "Primes Script",
    "description": "Create primes.py",
    "type": "task",
    "dependencies": []
  }
]`, nil
	}

	// 2. Coding Agent (Primes Scenario)
	// The prompt will describe the task: "Calculate all prime numbers less than 10,000..."
	if strings.Contains(prompt, "primes.py") || strings.Contains(prompt, "PRIMES") || strings.Contains(prompt, "prime number") {
		// Return a response that creates the file using a bash block
		return `
I will implement the prime number script as requested.

` + "```bash" + `
cat << 'EOF' > primes.py
import json

def sieve(n):
    primes = []
    is_prime = [True] * n
    is_prime[0] = is_prime[1] = False
    for p in range(2, n):
        if is_prime[p]:
            primes.append(p)
            for i in range(p * p, n, p):
                is_prime[i] = False
    return primes

with open("primes.json", "w") as f:
    json.dump({"primes": sieve(10000)}, f)
EOF

# Verify output
python3 primes.py
` + "```" + `

The script 'primes.json' has been generated.
`, nil
	}

	// 3. QA / Manager / Review
	// Often asks to review code or sign off
	if strings.Contains(prompt, "QA") || strings.Contains(prompt, "review") || strings.Contains(prompt, "sign off") {
		return `
The code looks correct and meets the requirements.
There are no bugs.
Validation passed.
I approve this change.
`, nil
	}

    // Default echo for debugging
	return fmt.Sprintf("Mock response to: %s...", truncateString(prompt, 50)), nil
}

func (m *MockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	resp, err := m.Send(ctx, prompt)
	if err == nil && onChunk != nil {
		onChunk(resp)
	}
	return resp, err
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
