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
// It also handles specific triggers for E2E tests (ticket generation, implementation, etc.)
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// 1. Ticket Generation Trigger
	// If the prompt is asking to generate a ticket plan from a spec
	if strings.Contains(prompt, "generate-from-spec") || strings.Contains(prompt, "Technical Program Manager") {
		return `[
  {
    "id": "req-primes",
    "title": "Implement Primes",
    "type": "Task",
    "description": "Implement a Python script to find prime numbers.",
    "status": "TODO"
  }
]`, nil
	}

	// 2. Implementation Trigger (primes.py)
	// If the prompt is asking to implement the primes feature
	if strings.Contains(prompt, "primes.py") || strings.Contains(prompt, "req-primes") {
		return `#!/bin/bash
cat << 'EOF' > primes.py
def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

import json
primes = [i for i in range(1, 101) if is_prime(i)]
print(json.dumps(primes))
with open('primes.json', 'w') as f:
    json.dump(primes, f)
EOF

# Ensure git user is set for commit
git config user.email "mock@example.com"
git config user.name "Mock Agent"

git add primes.py
git commit -m "Implement primes.py" || echo "Nothing to commit"

# Signal completion
if command -v agent-bridge &> /dev/null; then
    agent-bridge feature set req-primes --status done
fi
`, nil
	}

	// 3. Initialization Trigger
	// If the prompt is asking to initialize the workspace or extract features
	if strings.Contains(prompt, "Initialize") || strings.Contains(prompt, "feature_list.json") {
		return `#!/bin/bash
cat << 'EOF' > feature_list.json
[
  {
    "id": "req-primes",
    "title": "Implement Primes",
    "status": "todo"
  }
]
EOF
`, nil
	}

	// 4. QA / Manager Trigger
	// If the prompt is for QA or Manager review, always pass
	if strings.Contains(prompt, "QA AGENT") || strings.Contains(prompt, "PROJECT MANAGER") {
		return "QA_PASSED: true\nAPPROVED: true\nReason: Mock verification passed.", nil
	}

	// Default response
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
