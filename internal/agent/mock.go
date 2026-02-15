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
// It returns a mock response that acknowledges the prompt or specific JSON for TPM tasks
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	lowerPrompt := strings.ToLower(prompt)

	// Heuristic for 'Git Lead' phase
	if strings.Contains(lowerPrompt, "git lead") {
		return "git checkout -b feature/primes-implementation", nil
	}

	// Heuristic for 'prime-python' coding phase
	if strings.Contains(lowerPrompt, "id:[primes]") ||
		strings.Contains(lowerPrompt, "generate primes") ||
		strings.Contains(lowerPrompt, "primes.json") ||
		(strings.Contains(lowerPrompt, "prime") && strings.Contains(lowerPrompt, "python")) {
		// Only trigger if NOT TPM
		if !strings.Contains(lowerPrompt, "technical program manager") {
			return `
#!/bin/bash
cat <<EOF > primes.py
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

if __name__ == "__main__":
    import sys
    n = int(sys.argv[1]) if len(sys.argv) > 1 else 10
    print(generate_primes(n))
EOF
`, nil
		}
	}

	// Heuristic for TPM / Architect (JSON Ticket Generation)
	if strings.Contains(lowerPrompt, "json") &&
		(strings.Contains(lowerPrompt, "technical program manager") ||
			strings.Contains(lowerPrompt, "architect") ||
			strings.Contains(lowerPrompt, "tpm")) {

		return `
[
  {
    "id": "PRIMES",
    "title": "Implement Primes Generator",
    "description": "Create a python script that generates prime numbers.",
    "type": "Task",
    "dependencies": [],
    "repo_url": "https://github.com/process-failed-successfully/recac-jira-e2e"
  }
]
`, nil
	}

	// Heuristic for Git Commit Agent
	if strings.Contains(lowerPrompt, "commit message") {
		return "feat: Implement primes.py", nil
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
