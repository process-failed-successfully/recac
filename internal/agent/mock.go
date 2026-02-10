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

	// Heuristics for Smoke Tests and CI

	// 1. Initializer Agent
	// Detects "INITIALIZER AGENT" or "You are the FIRST agent"
	if strings.Contains(prompt, "INITIALIZER AGENT") || strings.Contains(prompt, "FIRST agent") {
		return `
I will set up the environment and create the feature list as requested.

` + "```bash" + `
# Initializer Setup
echo "Setting up environment..."
touch .env

# Create feature list in DB via agent-bridge
cat << 'EOF' | agent-bridge import --project "${RECAC_PROJECT_ID}"
{
  "project_name": "Test Project",
  "features": [
    {
      "id": "req-primes",
      "description": "Implement prime number generator",
      "priority": "MVP",
      "status": "pending",
      "steps": ["Run python3 primes.py", "Check output"],
      "dependencies": {"exclusive_write_paths": ["primes.py"]}
    }
  ]
}
EOF
` + "```", nil
	}

	// 2. Coding Agent
	if strings.Contains(prompt, "CODING AGENT") || strings.Contains(prompt, "Code State") {
		// No-Op Breaker: If task is "NONE_ALL_COMPLETE" (from SelectPrompt), return text only to trigger ErrNoOp
		if strings.Contains(prompt, "All features are marked as done") || strings.Contains(prompt, "NONE_ALL_COMPLETE") {
			return "Ack. Waiting for completion signal.", nil
		}

		// Loop Breaker: If nothing to commit, we are done
		if strings.Contains(prompt, "nothing to commit") || strings.Contains(prompt, "working tree clean") {
			return `
Everything looks good. I will signal QA.

` + "```bash" + `
agent-bridge signal --privileged QA_PASSED true
` + "```", nil
		}

		// Implement Primes (Standard Smoke Test Task)
		return `
I will implement the prime number generator in python.

` + "```bash" + `
# Implement primes.py
cat << 'EOF' > primes.py
def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

if __name__ == "__main__":
    primes = [p for p in range(100) if is_prime(p)]
    print(primes)
EOF

# Commit changes
git add primes.py
git commit -m "Implement prime number generator" || echo "Nothing to commit"
git push

# Mark feature as done (Crucial for loop termination)
agent-bridge feature set req-primes --status done --passes true || echo "Agent bridge not available"
` + "```", nil
	}

	// 3. QA Agent
	if strings.Contains(prompt, "QA AGENT") {
		return `
QA Verification Passed.

` + "```bash" + `
agent-bridge signal --privileged QA_PASSED true
` + "```", nil
	}

	// 4. Manager
	if strings.Contains(prompt, "Manager") || strings.Contains(prompt, "manager_review") {
		return `
Project Approved.

` + "```bash" + `
agent-bridge signal --privileged PROJECT_SIGNED_OFF true
` + "```", nil
	}

	// Default Mock Response (for unit tests expecting simple output)
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
