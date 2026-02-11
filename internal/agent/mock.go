package agent

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// MockAgent is a smart mock agent for testing and mock mode
// It returns predefined responses based on heuristics to simulate a real agent
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

	// 1. TPM Role (Planning)
	if strings.Contains(prompt, "You are an expert Technical Program Manager (TPM)") {
		return m.handleTPM(prompt)
	}

	// 2. Initializer Role
	if strings.Contains(prompt, "## YOUR ROLE - INITIALIZER AGENT") {
		return m.handleInitializer(prompt)
	}

	// 3. QA Role
	// Check strictly first to avoid confusion with Coding Agent mentioning QA
	if strings.Contains(prompt, "## YOUR ROLE - QA AGENT") {
		return "agent-bridge signal QA_PASSED true", nil
	}

	// 4. Manager Role
	if strings.Contains(prompt, "## YOUR ROLE - PROJECT MANAGER") {
		return "agent-bridge signal PROJECT_SIGNED_OFF true", nil
	}

	// 5. Coding Agent Role (Last as it's most generic)
	if strings.Contains(prompt, "## YOUR ROLE - CODING AGENT") ||
	   strings.Contains(prompt, "primes.py") ||
	   strings.Contains(prompt, "[PRIMES]") ||
	   strings.Contains(prompt, "Prime Number Script") {
		return m.handleCoding(prompt)
	}

	// Default fallback
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100))
	return response, nil
}

// SendStream implements the Agent interface
func (m *MockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	// Minimal delay to prevent timeouts but simulate async
	time.Sleep(10 * time.Millisecond)
	resp, err := m.Send(ctx, prompt)
	if err == nil && onChunk != nil {
		onChunk(resp)
	}
	return resp, err
}

func (m *MockAgent) handleTPM(prompt string) (string, error) {
	// Return a JSON plan for Prime Python
	jsonResponse := `[
  {
    "title": "ID:[PRIMES] Implement Prime Number Script",
    "description": "Implement a Python script to check for prime numbers. Repo: https://github.com/process-failed-successfully/recac-jira-e2e",
    "type": "Epic",
    "children": [
      {
        "title": "Create primes.py",
        "description": "Create a Python script that checks if a number is prime. Repo: https://github.com/process-failed-successfully/recac-jira-e2e",
        "type": "Story",
        "acceptance_criteria": [
          "Script runs without errors",
          "Correctly identifies prime numbers"
        ]
      }
    ]
  }
]`
	return "```json\n" + jsonResponse + "\n```", nil
}

func (m *MockAgent) handleInitializer(prompt string) (string, error) {
	// For prime-python, we might not need complex initialization,
	// but let's return a dummy feature list import just in case the runner expects it.
	// Note: % is escaped as %% to avoid printf formatting issues if any (though unlikely here)
	script := `
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
`
	return "```bash\n" + script + "\n```", nil
}

func (m *MockAgent) handleCoding(prompt string) (string, error) {
	// Return bash script to implement primes.py
	// We use %% for modulus to be safe if this string passes through a formatter
	script := `
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
`
	return "```bash\n" + script + "\n```", nil
}

// truncateString truncates a string to a maximum length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
