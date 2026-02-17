package agent

import (
	"context"
	"fmt"
	"strings"
)

// MockAgent is a smart mock agent for testing and mock mode
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

	// Heuristic: Planning Phase (Technical Program Manager)
	if strings.Contains(prompt, "Technical Program Manager") {
		// Mock response for Initializer phase (Plan Creation)
		// For PRIME scenario, return specific plan if detected
		if strings.Contains(prompt, "PRIME") || strings.Contains(prompt, "Prime") {
			return `[
  {
    "id": "req-implement-primes",
    "description": "Implement prime calculation logic in primes.py",
    "category": "functional",
    "priority": "critical",
    "status": "pending"
  },
  {
      "id": "req-verify-output",
      "description": "Output results to primes.json",
      "category": "functional",
      "priority": "critical",
      "status": "pending"
  }
]`, nil
		}
		// Default generic plan
		return `[{"id": "req-generic", "description": "Generic Implementation Task", "category": "functional", "priority": "medium", "status": "pending"}]`, nil
	}

	// Heuristic: Coding Agent (Bash script generation)
	// For PRIMES scenario specifically
	if strings.Contains(prompt, "ID:[PRIMES]") || strings.Contains(prompt, "Prime Number Script") || strings.Contains(prompt, "primes.py") {
		return `
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = [i for i in range(10000) if is_prime(i)]
with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
EOF

python3 primes.py || python primes.py

# Git config for CI environments
git config user.email "agent@recac.io" || true
git config user.name "RECAC Agent" || true

git add primes.py primes.json
git commit -m "Add primes script and output" || echo "Nothing to commit"

# Signal completion if using agent-bridge
agent-bridge feature set req-implement-primes --status done --passes true || true
agent-bridge feature set req-verify-output --status done --passes true || true
agent-bridge signal COMPLETED true || true
`, nil
	}

	// General Coding Agent fallback
	// If the prompt is asking for implementation but we don't recognize the task,
	// just mark everything as done to prevent infinite loops.
	if strings.Contains(prompt, "Coding Agent") || strings.Contains(prompt, "agent-bridge") {
		return `
# Mock Agent: Marking all features as done to proceed
agent-bridge feature list | jq -r '.features[].id' | xargs -I {} agent-bridge feature set {} --status done --passes true || true
agent-bridge signal COMPLETED true || true
`, nil
	}

	// Default generic response
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
