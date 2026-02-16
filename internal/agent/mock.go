package agent

import (
	"context"
	"fmt"
	"os"
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
// It returns a mock response that acknowledges the prompt or triggers heuristics
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	lowerPrompt := strings.ToLower(prompt)

	// 1. TPM Heuristic (Jira Ticket Generation)
	if strings.Contains(lowerPrompt, "technical program manager") || strings.Contains(lowerPrompt, "generate jira tickets") {
		// Return a valid JSON list of tickets for the Primes scenario
		return `[
  {
    "id": "PRIMES",
    "title": "Implement Primes Feature",
    "description": "Epic for primes feature",
    "type": "Epic"
  },
  {
    "id": "TASK-1",
    "title": "Implement is_prime function",
    "description": "Create a function to check for prime numbers",
    "type": "Task",
    "parent": "PRIMES"
  },
  {
    "id": "TASK-2",
    "title": "Create HTTP Handler",
    "description": "Create an API endpoint for primes",
    "type": "Task",
    "parent": "PRIMES"
  }
]`, nil
	}

	// 2. Manager/QA Heuristic (Sign-off)
	if strings.Contains(lowerPrompt, "qa report") || strings.Contains(lowerPrompt, "project manager") || strings.Contains(lowerPrompt, "manager review") {
		return "agent-bridge signal PROJECT_SIGNED_OFF true --privileged", nil
	}

	// 3. Primes/Coding Heuristic
	// Check prompt or env var for 'prime' context
	isPrimeContext := strings.Contains(lowerPrompt, "prime") ||
		strings.Contains(os.Getenv("RECAC_INJECTED_FEATURES"), "prime") ||
		strings.Contains(os.Getenv("RECAC_PROJECT_ID"), "PRIMES")

	if isPrimeContext {
		// Return commands to simulate work
		return `
# Creating implementation
cat <<EOF > primes.py
def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True
EOF

# Running tests (simulation)
echo "Running tests..."
python3 -c "from primes import is_prime; assert is_prime(7); assert not is_prime(4); print('Tests passed')"
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
	return s[:maxLen] + "..."
}
