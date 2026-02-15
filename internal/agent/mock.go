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

	// Heuristic: Check if the prompt is for the TPM / Architect agent (Generate Tickets)
	// The prompt typically starts with "You are an expert Technical Program Manager..."
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "generate ticket plan") {
		// Return a valid JSON plan for the 'prime-python' scenario or generic usage
		return `[
  {
    "title": "ID:[PRIMES] Prime Number Generator",
    "description": "Implement a Python script to generate prime numbers.",
    "type": "Story",
    "children": [],
    "acceptance_criteria": [
      "Must generate primes up to N",
      "Must output result to primes.json"
    ]
  }
]`, nil
	}

	// Heuristic: Check if the prompt is for the Coding Agent (Implement Code)
	// The prompt typically asks to "implement the following task" or similar
	// We need to return a bash block that writes the file and runs it,
	// because the executor looks for ```bash blocks to execute.
	if strings.Contains(prompt, "Implement a Python script") || strings.Contains(strings.ToLower(prompt), "primes") {
		return `I will create the python script to generate prime numbers.

` + "```bash" + `
cat <<EOF > primes.py
import json

def generate_primes(n):
    primes = []
    for num in range(2, n + 1):
        is_prime = True
        for i in range(2, int(num ** 0.5) + 1):
            if num % i == 0:
                is_prime = False
                break
        if is_prime:
            primes.append(num)
    return primes

if __name__ == '__main__':
    primes = generate_primes(100)
    with open('primes.json', 'w') as f:
        json.dump({'primes': primes}, f)
    print(f'Generated {len(primes)} primes.')
EOF

python3 primes.py
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
