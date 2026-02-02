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

	// Heuristic: If this looks like a Ticket Generation request (TPM), return valid JSON
	if isTicketGenerationPrompt(prompt) {
		// Return a hardcoded list of tickets expected by the 'generate-from-spec' command
		// The key must match what the E2E test expects (e.g. PRIMES)
		return `[
  {
    "id": "MOCK-1",
    "key": "PRIMES",
    "title": "Implement Prime Number Service",
    "description": "Create a Python service that calculates prime numbers.",
    "type": "Task",
    "dependencies": []
  }
]`, nil
	}

	// Heuristic for Implementation (Coding) - specific to E2E smoke tests
	if contains(strings.ToUpper(prompt), "PRIMES") || contains(strings.ToUpper(prompt), "PRIME NUMBER") {
		return `I will implement the prime number service in Python.
` + "```bash" + `
cat <<EOF > primes.py
def get_primes(n):
    primes = []
    for i in range(2, n + 1):
        is_prime = True
        for j in range(2, int(i ** 0.5) + 1):
            if i % j == 0:
                is_prime = False
                break
        if is_prime:
            primes.append(i)
    return primes

if __name__ == "__main__":
    print(get_primes(20))
EOF

# Verify it works
python3 primes.py

# Commit the change
git add primes.py
git commit -m "Implement prime number service" || echo "Nothing to commit"

# Signal completion
if command -v agent-bridge >/dev/null 2>&1; then
    agent-bridge signal COMPLETED true
fi
` + "```", nil
	}

	// Return a mock response that shows the agent received the prompt
	// This allows the session to run without requiring real API keys
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100))

	// Append a no-op bash block to prevent the "circuit breaker: no-op loop" error
	// The agent runner expects actionable commands to consider the turn "productive".
	response += "\n\nI will wait for further instructions.\n```bash\n# No-op to prevent circuit breaker\n: \n```"

	return response, nil
}

func isTicketGenerationPrompt(prompt string) bool {
	// Check for keywords used in the TPM prompt
	// This is fragile but necessary for the mock to support the 'generate-from-spec' flow
	// which expects strict JSON output.
	return len(prompt) > 0 && (contains(prompt, "Technical Program Manager") || contains(prompt, "generate-from-spec"))
}

func contains(s, substr string) bool {
	// Simple containment check, case-insensitive could be better but sticking to stdlib
	for i := 0; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
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
