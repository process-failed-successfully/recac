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

	// Heuristics for E2E Smoke Tests
	// 1. TPM Agent (Expects JSON for ticket generation)
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "TPM") {
		// Return a valid JSON array of tickets
		return `[
  {
    "title": "Implement Primes Script",
    "description": "Create a Python script that calculates prime numbers up to 100.",
    "type": "Task",
    "acceptance_criteria": ["Script runs successfully", "Output contains prime numbers"],
    "priority": "High",
    "id": "ID:[PRIMES]"
  }
]`, nil
	}

	// 2. Initializer Agent (Expects bash script to import features)
	if strings.Contains(prompt, "INITIALIZER AGENT") || strings.Contains(prompt, "feature_list.json") {
		return `
cat <<EOF > feature_list.json
[
  {
    "id": "req-script-primes-py-exists",
    "name": "Primes Script Exists",
    "description": "The primes.py script must exist",
    "status": "pending"
  }
]
EOF
agent-bridge import < feature_list.json
`, nil
	}

	// 3. Coding Agent (Expects implementation)
	if strings.Contains(prompt, "CODING AGENT") || strings.Contains(prompt, "Primes") || strings.Contains(prompt, "primes.py") {
		return `
cat <<EOF > primes.py
def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

print([n for n in range(100) if is_prime(n)])
EOF

# Mark feature as passed
agent-bridge feature set req-script-primes-py-exists passed || echo "Feature not found"

# Commit changes (guarded)
git add primes.py
git commit -m "Implement primes script" || echo "Nothing to commit"
`, nil
	}

	// 4. QA Agent (Expects pass signal)
	if strings.Contains(prompt, "QA AGENT") {
		return "QA_PASSED", nil
	}

	// 5. Project Manager (Expects sign-off)
	if strings.Contains(prompt, "PROJECT MANAGER") {
		return "PROJECT_SIGNED_OFF", nil
	}

	// Default response
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
