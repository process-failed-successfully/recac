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

	// TPM Mode (Planning Phase)
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "TPM") {
		return `[
  {
    "title": "ID:[PRIMES] Implement Prime Number Generator",
    "description": "Create a python script primes.py that prints prime numbers up to 100.",
    "type": "Task",
    "status": "In Progress"
  }
]`, nil
	}

	// Primes Scenario (Coding Phase)
	if strings.Contains(prompt, "ID:[PRIMES]") || strings.Contains(prompt, "primes.py") {
		// Check for completion first to avoid infinite loops
		if strings.Contains(prompt, "nothing to commit") || strings.Contains(prompt, "working tree clean") {
			return `
echo "Task ID:[PRIMES] completed."
echo '[{"key": "ID:[PRIMES]", "summary": "Implement Prime Number Generator", "status": "Done"}]' | agent-bridge import
agent-bridge feature set "ID:[PRIMES]" "status" "Done"
exit 0
`, nil
		}

		return `
cat <<EOF > primes.py
def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

if __name__ == "__main__":
    for i in range(100):
        if is_prime(i):
            print(i)
EOF

python3 primes.py
git add primes.py
git commit -m "Implement primes.py"
git status
`, nil
	}

	// Default Mock Response
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
