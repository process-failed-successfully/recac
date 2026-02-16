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

	// 1. TPM Phase: JSON Response
	if strings.Contains(strings.ToLower(prompt), "technical program manager") || strings.Contains(prompt, "ID:[PRIMES]") {
		return `{
  "summary": "Implement prime number generator",
  "features": [
    {
      "id": "req-primes",
      "description": "Implement a Python script that generates prime numbers up to 10000",
      "dependencies": []
    }
  ],
  "risks": []
}`, nil
	}

	// 2. Coding Phase: Bash Script Response
	if strings.Contains(strings.ToLower(prompt), "implement") || strings.Contains(strings.ToLower(prompt), "write") || strings.Contains(strings.ToLower(prompt), "create") {
		return `
Here is the implementation for the prime number generator.

I will create a Python script ` + "`primes.py`" + ` that calculates primes up to 10,000.

` + "```bash" + `
cat <<EOF > primes.py
import json

def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

primes = [x for x in range(10000) if is_prime(x)]
print(json.dumps({"primes": primes}))
EOF

# Run it to verify
python3 primes.py > primes.json
` + "```" + `
`, nil
	}

	// 3. Testing/Verify Phase: "Task Completed"
	if strings.Contains(strings.ToLower(prompt), "ran 2 tests") && strings.Contains(strings.ToLower(prompt), "ok") {
		return "Task completed. The tests passed successfully.", nil
	}

	// Default Response
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
