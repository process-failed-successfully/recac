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
// It returns a mock response based on the prompt content
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// 1. TPM Agent (Ticket Generation)
	if strings.Contains(prompt, "Technical Program Manager") {
		return `[
  {
    "title": "ID:[PRIMES] Implement Prime Number Generator",
    "description": "Implement a Python script to generate prime numbers.",
    "type": "Story",
    "acceptance_criteria": [
      "Script generates primes up to N",
      "Performance is acceptable"
    ],
    "children": []
  }
]`, nil
	}

	// 2. Coding Agent (Implementation)
	if strings.Contains(prompt, "Prime") || strings.Contains(prompt, "Implementation") {
		return `Here is the implementation:

` + "```bash" + `
cat << 'EOF' > primes.py
def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

print([x for x in range(20) if is_prime(x)])
EOF

python3 primes.py
` + "```" + `
`, nil
	}

	// 3. QA/Review Agent (and Manager)
	if strings.Contains(strings.ToUpper(prompt), "QA") || strings.Contains(strings.ToUpper(prompt), "REVIEW") || strings.Contains(strings.ToUpper(prompt), "MANAGER") {
		// Manager/QA needs to approve.
		// For Manager, we explicitly return the signal command to be robust, though the runner also accepts ratio checks.
		return `LGTM. The code looks correct and meets the requirements.
` + "```bash" + `
agent-bridge set-signal PROJECT_SIGNED_OFF true
` + "```" + `
`, nil
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
