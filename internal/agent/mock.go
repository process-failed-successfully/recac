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

	// Heuristics for prime-python scenario
	if strings.Contains(prompt, "Create a JSON object containing a feature list") {
		// Planner response
		return `
{
  "features": [
    {
      "id": "[PRIMES]",
      "description": "Implement prime number calculation script",
      "type": "Task"
    }
  ]
}`, nil
	}

	if strings.Contains(prompt, "primes.py") {
		// Implementation response
		return `I will create the primes.py script and the output file.

` + "```bash" + `
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n < 2: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = [x for x in range(10000) if is_prime(x)]

with open('primes.json', 'w') as f:
    json.dump({'primes': primes}, f)
EOF

python3 primes.py
git add primes.py primes.json
git commit -m "Implement primes script"
` + "```" + `
`, nil
	}

	// Default response with no-op bash block to prevent circuit breaker trip
	format := "%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...\n\n```bash\n# no-op\necho 'mock agent alive'\n```"
	response := fmt.Sprintf(format,
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
