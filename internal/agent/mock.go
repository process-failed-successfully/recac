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
	// Return a mock response that shows the agent received the prompt
	// This allows the session to run without requiring real API keys

	// Heuristic for "Plan Generation" prompt (e.g. Jira ticket creation)
	if isPlanningPrompt(prompt) {
		return `{
  "project_name": "Prime Number Generator",
  "features": [
    {
      "id": "PRIMES",
      "category": "functional",
      "description": "Calculate prime numbers using Python. Create a file named primes.py that prints prime numbers up to 100.",
      "status": "pending",
      "steps": [
        "Create primes.py",
        "Implement prime calculation logic",
        "Print primes to stdout"
      ],
      "dependencies": {
        "depends_on_ids": [],
        "exclusive_write_paths": [],
        "read_only_paths": []
      }
    }
  ]
}`, nil
	}

	// Heuristic for "Task Execution" prompt (e.g. actually writing code)
	// If the prompt mentions "primes.py" or the description from above, return code.
	if isImplementationPrompt(prompt) {
		return `Here is the solution:

` + "```bash" + `
# Create primes.py
cat << 'EOF' > primes.py
def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = [n for n in range(101) if is_prime(n)]
print(primes)
EOF

# Verify
python3 primes.py
` + "```" + `

Done.
`, nil
	}

	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100))
	return response, nil
}

func isPlanningPrompt(prompt string) bool {
	// Key phrase from internal/agent/prompts/templates/planner.md
	// "Create a JSON object containing a feature list"
	// Also checking specific project keywords might help if we have multiple scenarios
	// But for now, we assume the smoke test is running the prime-python scenario or generic.
	// However, the prompt might vary.
	// Let's use a very specific check for the CI smoke test case if possible, or generic json plan.
	return len(prompt) > 0 && (contains(prompt, "Create a JSON object containing a feature list") || contains(prompt, "ID:[PRIMES]"))
}

func isImplementationPrompt(prompt string) bool {
	return len(prompt) > 0 && (contains(prompt, "primes.py") || contains(prompt, "Calculate prime numbers"))
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
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
