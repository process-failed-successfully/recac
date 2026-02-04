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

	// 1. TPM Role: Ticket Generation
	// Heuristic: If prompt asks for JSON ticket plan (TPM role), return valid JSON
	if strings.Contains(prompt, "Technical Program Manager") && strings.Contains(prompt, "JSON") {
		return `{
  "id": "PRIMES",
  "project_name": "prime-python",
  "tickets": [
    {
      "title": "Implement Prime Number Generator",
      "description": "Create a Python script to generate prime numbers.",
      "type": "Task",
      "status": "Todo",
      "id": "PRIMES-1",
      "dependencies": []
    }
  ]
}`, nil
	}

	// 2. Implementation Role: Coding
	// Heuristic: Check for specific task title or keywords
	if strings.Contains(prompt, "Implement Prime Number Generator") ||
	   strings.Contains(prompt, "primes.py") ||
	   strings.Contains(prompt, "Calculate primes") {

		return `I will implement the prime number generator in Python and mark the task as complete.

` + "```bash" + `
cat << 'EOF' > primes.py
def is_prime(n):
    """Checks if a number is a prime number."""
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

if __name__ == "__main__":
    import sys
    if len(sys.argv) > 1:
        n = int(sys.argv[1])
        print(is_prime(n))
EOF

# Mark feature as implemented and passing to trigger completion
agent-bridge feature set PRIMES-1 --status implemented --passes true || echo "agent-bridge failed"
` + "```" + `

I have created the primes.py file and updated the feature status.
`, nil
	}

	// 3. QA Agent Role
	// Heuristic: "QA Agent" or verification keywords
	if strings.Contains(prompt, "QA Agent") || strings.Contains(prompt, "Quality Assurance") {
		return `I have verified the implementation. It looks correct.

` + "```bash" + `
python3 primes.py 7
# Signal QA success
agent-bridge signal QA_PASSED true || echo "agent-bridge failed"
` + "```" + `
`, nil
	}

	// 4. Manager Agent Role
	// Heuristic: "Manager" or "Project Manager" review
	if strings.Contains(prompt, "Manager Agent") || strings.Contains(prompt, "Project Manager") || strings.Contains(prompt, "QA Report") {
		return `The implementation and QA look good. I approve.

` + "```bash" + `
# Signal Project Sign-off
agent-bridge signal PROJECT_SIGNED_OFF true || echo "agent-bridge failed"
` + "```" + `
`, nil
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
