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

	// 1. TPM Heuristic (Planning)
	if strings.Contains(prompt, "Technical Program Manager (TPM)") || strings.Contains(prompt, "TPM") {
		return `[
  {
    "title": "ID:[PRIMES] Implement a Python script to calculate prime numbers",
    "description": "Create a script named primes.py that calculates and prints the first n prime numbers.",
    "type": "Task",
    "status": "TODO",
    "priority": "High"
  }
]`, nil
	}

	// 2. Coding Heuristic (Primes)
	// Triggers on "ID:[PRIMES]" or explicit mention of "primes.py"
	if strings.Contains(prompt, "ID:[PRIMES]") || strings.Contains(prompt, "primes.py") {
		script := `
cat <<EOF > primes.py
def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

count = 0
num = 2
while count < 10:
    if is_prime(num):
        print(num)
        count += 1
    num += 1
EOF

python3 primes.py

git add primes.py
git commit -m "Add primes.py script"
echo "Recac Finished"
`
		return script, nil
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
