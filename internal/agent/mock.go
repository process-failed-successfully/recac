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

	lowerPrompt := strings.ToLower(prompt)

	// Heuristic 1: Coding Task (Primes) - Used in smoke-test
	// We return a Markdown block with Python code and Bash commands to commit it.
	if strings.Contains(lowerPrompt, "prime") || strings.Contains(lowerPrompt, "python") {
		return `Here is a python script to print prime numbers:

` + "```python" + `
def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

for i in range(100):
    if is_prime(i):
        print(i)
` + "```" + `

To run this and commit it:
` + "```bash" + `
echo "Creating primes.py..."
cat <<EOF > primes.py
def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

for i in range(100):
    if is_prime(i):
        print(i)
EOF

python3 primes.py > primes.txt
git add primes.py primes.txt
git commit -m "Add primes.py"
` + "```", nil
	}

	// Heuristic 2: Planning Task - Returns JSON tickets
	if strings.Contains(lowerPrompt, "ticket") || strings.Contains(lowerPrompt, "plan") {
		return `[{"title": "Implement Primes", "description": "Write a python script for primes", "priority": "High"}]`, nil
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
