package agent

import (
	"context"
	"fmt"
	"strings"
)

// MockAgent is a smart mock agent for testing and mock mode
// It includes heuristics to pass E2E scenarios like prime-python
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

	// Heuristics for Smoke Tests (prime-python)

	// 1. Ticket Generation (TPM Agent)
	// Check this FIRST to avoid confusion with implementation keywords that might appear in the spec.
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "tickets") {
		return `
[
  {
    "title": "ID:[PRIMES] Implement Prime Number Calculator",
    "description": "Implement a Python script to calculate prime numbers.",
    "type": "Story",
    "acceptance_criteria": [
      "Must correctly identify prime numbers",
      "Must print primes up to 20"
    ],
    "children": []
  }
]
`, nil
	}

	// 2. QA / Verification Phase
	if strings.Contains(prompt, "YOUR ROLE - QA AGENT") {
		return "```bash\nagent-bridge signal QA_PASSED true\n```\nQA Passed.", nil
	}

	// 3. Manager Sign-off
	if strings.Contains(prompt, "PROJECT MANAGER") {
		return "```bash\nagent-bridge signal PROJECT_SIGNED_OFF true\n```\nProject Approved.", nil
	}

	// 4. Implementation Phase (primes.py)
	if strings.Contains(prompt, "Calculate primes") || strings.Contains(prompt, "[PRIMES]") {
		return `
Sure, I will create a python script to calculate primes.

` + "```bash" + `
cat <<EOF > primes.py
def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

print([x for x in range(20) if is_prime(x)])
EOF

# Signal feature completion
agent-bridge feature set req-must-correctly-identify-prime- --status done --passes true
agent-bridge feature set req-must-print-primes-up-to-20 --status done --passes true
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
