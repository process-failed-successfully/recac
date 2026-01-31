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

	// Smoke Test Scenario: prime-python

	// 1. Manager Role: Generate Tickets
	if strings.Contains(prompt, "app_spec.txt") && strings.Contains(prompt, "prime numbers") && (strings.Contains(prompt, "Ticket") || strings.Contains(prompt, "Jira")) {
		return `[
  {
    "title": "ID:[PRIMES] Prime Number Script",
    "description": "Implement a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to a file named 'primes.json'. The JSON format must have a single key 'primes' containing the list of integers. The script MUST be named 'primes.py'. The output file MUST be named 'primes.json'. Implement prime calculation logic in primes.py, output results to primes.json, validate that the output file contains a 'primes' list, verify that exactly 1229 primes are calculated, and commit primes.json to the repository. Repo: https://github.com/process-failed-successfully/recac-jira-e2e",
    "type": "Epic",
    "children": [
      {
        "title": "Implement primes.py",
        "description": "Create the primes.py script.",
        "type": "Task"
      }
    ]
  }
]`, nil
	}

	// 2. Agent Role: Implementation
	if strings.Contains(prompt, "Software Engineer") {
		// Heuristic state machine based on what's in the prompt (history/output)

		// Step 1: Initial State -> List files
		if !strings.Contains(prompt, "ls -la") {
			return "ls -la", nil
		}

		// Step 2: Files listed -> Read spec
		if strings.Contains(prompt, "ls -la") && !strings.Contains(prompt, "cat app_spec.txt") {
			return "cat app_spec.txt", nil
		}

		// Step 3: Spec read -> Write script
		if strings.Contains(prompt, "cat app_spec.txt") && !strings.Contains(prompt, "cat << 'EOF' > primes.py") {
			return `cat << 'EOF' > primes.py
import json

def calculate_primes(n):
    primes = []
    for num in range(2, n):
        if all(num % i != 0 for i in range(2, int(num ** 0.5) + 1)):
            primes.append(num)
    return primes

def main():
    primes = calculate_primes(10000)
    with open('primes.json', 'w') as f:
        json.dump({'primes': primes}, f)

if __name__ == '__main__':
    main()
EOF`, nil
		}

		// Step 4: Script written -> Run script and test
		if strings.Contains(prompt, "cat << 'EOF' > primes.py") && !strings.Contains(prompt, "python3 primes.py") {
			return "python3 primes.py", nil
		}

		// Step 5: Script ran -> Report Success (Agent Bridge)
		if strings.Contains(prompt, "python3 primes.py") && !strings.Contains(prompt, "agent-bridge feature set") {
			// In smoke test, we assume features are injected.
			// The logs showed: "req-the-script-primes-py-is-implem", etc.
			return `agent-bridge feature set req-the-script-primes-py-is-implem --status done --passes true
agent-bridge feature set req-the-output-is-written-to-a-fil --status done --passes true
agent-bridge feature set req-the-primes-json-file-contains- --status done --passes true
agent-bridge feature set req-the-list-of-primes-in-primes-j --status done --passes true`, nil
		}

		// Step 6: Bridge called -> Commit
		if strings.Contains(prompt, "agent-bridge feature set") && !strings.Contains(prompt, "git commit") {
			return `git add .
git commit -m "feat: implement primes.py"`, nil
		}

		// Step 7: Commit done -> No-op (loop preventer)
		if strings.Contains(prompt, "git commit") {
			return "# task complete", nil
		}
	}

	// Default fallback
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
