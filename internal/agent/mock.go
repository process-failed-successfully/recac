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

	// Heuristic Role Detection for E2E scenarios

	// 1. Technical Program Manager (TPM) - Planning Phase
	// Expectation: Return a JSON list of features/tickets
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "generate a list of features") {
		// Return a valid JSON list of tickets as expected by the Jira generator
		// The format must match what `recac jira generate-from-spec` expects
		return `Here is the plan:
` + "```json" + `
[
  {
    "title": "ID:[PRIMES] Implement Primes",
    "description": "Implement a Python script that calculates prime numbers up to a given limit.",
    "type": "Task",
    "priority": "High",
    "dependencies": [],
    "acceptance_criteria": [
      "The script should accept a command-line argument for the limit.",
      "The script should print the prime numbers to stdout."
    ]
  }
]
` + "```", nil
	}

	// 2. Initializer Agent - Bootstrap Phase
	// Expectation: Return a bash script to import features into the system
	if strings.Contains(prompt, "INITIALIZER AGENT") {
		return `I will initialize the system with the feature list.

` + "```bash" + `
#!/bin/bash
# Import the feature list into the agent bridge
cat << 'EOF' | agent-bridge import
{
  "features": [
    {
      "title": "ID:[PRIMES] Implement Primes",
      "description": "Implement a Python script that calculates prime numbers up to a given limit.",
      "status": "todo"
    }
  ]
}
EOF
` + "```", nil
	}

	// 3. Coding Agent - Implementation Phase
	// Expectation: Return the implementation file (primes.py) and mark task as done
	if strings.Contains(prompt, "CODING AGENT") || strings.Contains(prompt, "Prime Number Script") || strings.Contains(prompt, "primes.py") {
		return `I will create the primes script and mark the task as complete.

` + "```bash" + `
#!/bin/bash
# Create the primes.py script
cat << 'EOF' > primes.py
import sys

def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

def main():
    if len(sys.argv) > 1:
        limit = int(sys.argv[1])
    else:
        limit = 100

    primes = [i for i in range(limit + 1) if is_prime(i)]
    print(primes)

if __name__ == "__main__":
    main()
EOF

# Run the script to verify
python3 primes.py 20

# Mark the task as completed
agent-bridge feature update PRIMES --status completed

# Signal completion
agent-bridge signal PROJECT_SIGNED_OFF true --privileged || touch PROJECT_SIGNED_OFF
` + "```", nil
	}

	// 4. QA Agent - Verification Phase
	// Expectation: Verify and Sign-off
	if strings.Contains(prompt, "QA AGENT") {
		return `The implementation looks correct.
` + "```json" + `
true
` + "```", nil
	}

	// Default Fallback
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100))
	return response, nil
}

// SendStream implements the Agent interface
func (m *MockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	// For mock, just send the whole response as one chunk (or simulate streaming if needed)
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
