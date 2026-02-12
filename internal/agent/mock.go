package agent

import (
	"context"
	"fmt"
	"strings"
)

// MockAgent is a smart mock agent for testing and mock mode
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
// It also implements heuristics to simulate specific agents (TPM, Coding, Initializer, QA)
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// 1. TPM Agent (Scenario Generation)
	if strings.Contains(prompt, "Technical Program Manager") {
		// Return JSON array of tickets for the scenario
		return "```json\n" + `
[
  {
    "summary": "Implement Prime Number Generator",
    "description": "Create a Python script that generates prime numbers. ID:[PRIMES]",
    "issuetype": "Story",
    "labels": ["recac-agent"]
  }
]
` + "\n```", nil
	}

	// 2. Initializer Agent (Feature Loading)
	if strings.Contains(prompt, "INITIALIZER AGENT") || strings.Contains(prompt, "You are an Initializer Agent") {
		return "```bash\necho '[{\"id\":\"PRIMES\", \"description\":\"Implement Prime Number Generator\", \"status\":\"todo\"}]' | agent-bridge import\n```", nil
	}

	// 3. Coding Agent (Implementation)
	if strings.Contains(prompt, "CODING AGENT") || strings.Contains(prompt, "Coding Agent") || strings.Contains(prompt, "prime") {
		// Detect if task is already completed based on prompt context (simple heuristic)
		if strings.Contains(prompt, "Add primes script") {
			return "Task Completed", nil
		}

		return `I will implement the prime number generator.

` + "```python" + `
def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

if __name__ == "__main__":
    import sys
    print([x for x in range(100) if is_prime(x)])
` + "```\n\n```bash" + `
cat <<EOF > primes.py
def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

if __name__ == "__main__":
    import sys
    print([x for x in range(100) if is_prime(x)])
EOF

agent-bridge feature set PRIMES --status completed --passes true
` + "```", nil
	}

	// 4. QA Agent
	if strings.Contains(prompt, "Approve or Reject") {
		return "QA_PASSED", nil
	}
	if strings.Contains(prompt, "QA Agent") {
		return "LGTM", nil
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
