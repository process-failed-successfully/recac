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
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// Heuristic to detect Plan request (from cmd/recac/plan.go)
	if strings.Contains(prompt, "app_spec.txt") || strings.Contains(prompt, "Feature Implementation Plan") || strings.Contains(prompt, "spec") {
		// Return a valid JSON FeatureList for the prime-python scenario
		// We assume the spec asks for prime numbers
		return `{
  "project_name": "prime-calculator",
  "features": [
    {
      "id": "PRIMES",
      "category": "core",
      "priority": "MVP",
      "description": "Implement a Python script to calculate prime numbers",
      "status": "pending",
      "passes": false,
      "steps": [
        "Create primes.py",
        "Implement is_prime function",
        "Add main block to print primes up to 100"
      ],
      "dependencies": {
        "exclusive_write_paths": ["primes.py"]
      }
    }
  ]
}`, nil
	}

	// Heuristic to detect Agent implementation request
	// The agent receives the task ID/Description.
	if strings.Contains(prompt, "PRIMES") || strings.Contains(prompt, "prime numbers") {
		// Return a bash script to implement the code
		return `
Explanation: I will create a Python script to calculate prime numbers.

` + "```bash" + `
cat <<EOF > primes.py
def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

if __name__ == "__main__":
    print("Primes up to 100:")
    for i in range(101):
        if is_prime(i):
            print(i, end=" ")
    print()
EOF
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
