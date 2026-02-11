package agent

import (
	"context"
	"fmt"
	"strings"
	"time"
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

	// Logging for CI debugging
	fmt.Printf("[MockAgent] Received prompt (%d chars): %s...\n", len(prompt), truncateString(prompt, 50))

	// Heuristics for different roles/tasks

	// 1. Initializer Agent
	if strings.Contains(prompt, "## YOUR ROLE - INITIALIZER AGENT") {
		fmt.Println("[MockAgent] Detected Initializer Role")
		return `
echo "Initializing..."
cat << 'EOF' | agent-bridge import
{
  "features": [
    {
      "name": "Prime Check",
      "description": "Check if a number is prime",
      "status": "todo"
    }
  ]
}
EOF
`, nil
	}

	// 2. Project Manager / TPM
	if strings.Contains(prompt, "## YOUR ROLE - PROJECT MANAGER") || strings.Contains(prompt, "You are an expert Technical Program Manager (TPM)") {
		fmt.Println("[MockAgent] Detected Manager Role")
		// If asked to generate a plan (usually has "generate a plan" or similar, or just context)
		// For now, if it sees features, it might just approve or plan.
		// If prompt asks for "tickets", return JSON list.
		// If prompt is reviewing status, check if features are done.

		// Heuristic: If prompt mentions "QA_PASSED", sign off.
		if strings.Contains(prompt, "QA_PASSED") {
			return `Manager approved. Project signed off.
agent-bridge signal PROJECT_SIGNED_OFF true --privileged || touch PROJECT_SIGNED_OFF
`, nil
		}

		// Default TPM response: create tickets or just acknowledge.
		// The logs showed it returning JSON for tickets.
		return `[
  {"title": "ID:[PRIMES] Implement Primes", "description": "Implement is_prime function", "status": "todo"}
]`, nil
	}

	// 3. Coding Agent
	if strings.Contains(prompt, "## YOUR ROLE - CODING AGENT") || strings.Contains(strings.ToUpper(prompt), "[PRIMES]") {
		fmt.Println("[MockAgent] Detected Coding Role")
		// Specific heuristic for Primes task
		if strings.Contains(prompt, "primes.py") || strings.Contains(prompt, "Prime Check") {
			return `
echo "Implementing primes.py..."
cat << 'EOF' > primes.py
import sys

def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

if __name__ == "__main__":
    if len(sys.argv) > 1:
        if is_prime(int(sys.argv[1])):
            print("Prime")
        else:
            print("Not Prime")
EOF

# Mark as completed (using ID from scenario)
agent-bridge feature update "[PRIMES]" --status completed || true
`, nil
		}

		// Generic Coding response if not matched specific task
		return `echo "Coding agent working..."`, nil
	}

	// 4. QA Agent
	if strings.Contains(prompt, "## YOUR ROLE - QA AGENT") {
		fmt.Println("[MockAgent] Detected QA Role")
		// Always pass for smoke tests
		return `
echo "QA Passed"
agent-bridge signal QA_PASSED true || touch QA_PASSED
`, nil
	}

	// Fallback
	fmt.Println("[MockAgent] No specific role detected, returning generic response.")
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100))
	return response, nil
}

// SendStream implements the Agent interface
func (m *MockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	// Add small delay to simulate streaming and prevent tight loops in tests if logic is too fast
	time.Sleep(10 * time.Millisecond)

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
