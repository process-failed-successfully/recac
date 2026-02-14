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

	// Heuristic: Check if this is a Planning / Ticket Generation request
	// The prompt typically starts with "You are an expert Technical Program Manager (TPM)..."
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "TPM") {
		// Return a JSON list of tickets for the mock scenario
		// This matches the schema expected by `ticketNode` in `cmd/recac/jira.go`
		return `[
  {
    "title": "ID:[PRIMES] Implement Prime Number Generator",
    "description": "Create a Python script that generates prime numbers.",
    "type": "Story",
    "acceptance_criteria": [
      "Must be written in Python",
      "Must handle input n to generate first n primes"
    ],
    "children": [
      {
        "title": "ID:[PRIMES-1] Create primes.py",
        "description": "Repo: https://github.com/process-failed-successfully/recac-jira-e2e\nImplement the basic prime checking logic.",
        "type": "Subtask"
      },
      {
        "title": "ID:[PRIMES-2] Add CLI interface",
        "description": "Repo: https://github.com/process-failed-successfully/recac-jira-e2e\nAdd argparse to handle user input.",
        "type": "Subtask"
      }
    ]
  }
]`, nil
	}

	// Heuristic: Execution Phase - Primes Scenario
	// Triggered when the agent is executing the tasks created above
	if (strings.Contains(prompt, "ID:[PRIMES") || strings.Contains(prompt, "Prime Number Generator")) &&
		(strings.Contains(prompt, "## YOUR ROLE - CODING AGENT") || strings.Contains(prompt, "Role: Agent")) {

		return `#!/bin/bash
# Mock implementation of Prime Number Generator
echo "Creating primes.py..."
cat <<EOF > primes.py
import json
import sys

def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

def generate_primes(n):
    primes = []
    num = 2
    while len(primes) < n:
        if is_prime(num):
            primes.append(num)
        num += 1
    return primes

if __name__ == "__main__":
    count = 10
    if len(sys.argv) > 1:
        count = int(sys.argv[1])

    primes = generate_primes(count)
    print(f"Generated {len(primes)} primes")

    with open("primes.json", "w") as f:
        json.dump({"primes": primes}, f)
EOF

# Run it to generate the artifact checked by tests
python3 primes.py 5

# Signal completion
# We must first ensure the feature exists locally if we are in a fresh container
# Then we update status and signal project completion
agent-bridge import <<JSON
[
  {"id": "MFLP-12444", "description": "ID:[PRIMES] Implement Prime Number Generator", "status": "todo"},
  {"id": "MFLP-12445", "description": "ID:[PRIMES-1] Create primes.py", "status": "todo"},
  {"id": "MFLP-12446", "description": "ID:[PRIMES-2] Add CLI interface", "status": "todo"}
]
JSON

# Mark tasks as done (using ID matching logic would be better, but for smoke test we just signal project done)
agent-bridge signal PROJECT_SIGNED_OFF true --privileged
echo "Task completed. No further changes needed."
`, nil
	}

	// Return a mock response that shows the agent received the prompt
	// This allows the session to run without requiring real API keys
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
