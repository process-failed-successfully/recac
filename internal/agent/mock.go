package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// MockAgent is a smart mock agent for testing and mock mode
// It returns heuristic-based responses to simulate a real agent in E2E tests
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

	// Heuristics for E2E scenarios

	// 1. Initializer Agent
	// Checks for "initializer agent" in prompt (case-insensitive)
	if strings.Contains(strings.ToLower(prompt), "initializer agent") {
		// Return a bash script that imports features
		return `
Here is the initialization script:

` + "```bash" + `
#!/bin/bash
# Mock Initializer Script

# Import features into the DB using agent-bridge
cat << 'EOF' | agent-bridge import
{
  "project_name": "recac-e2e",
  "features": [
    {
      "id": "PRIMES",
      "name": "Primes Calculation",
      "description": "Calculate prime numbers",
      "priority": "High",
      "status": "pending"
    }
  ]
}
EOF

echo "Initialization complete."
` + "```" + `
`, nil
	}

	// 2. TPM / Planner (Jira Generation)
	// Checks for "TPM" or "ticket" generation context
	// The prompt from 'recac plan' or 'jira generate' usually contains "spec"
	if strings.Contains(strings.ToLower(prompt), "tpm") || (strings.Contains(prompt, "spec") && strings.Contains(prompt, "tickets")) {
		// Return JSON with ID:[PRIMES] to match the E2E expectation
		tickets := []map[string]string{
			{
				"title":       "Implement Primes Calculation ID:[PRIMES]",
				"description": "Create a python script that calculates primes and writes to primes.json",
				"type":        "Task",
			},
		}
		data, _ := json.Marshal(tickets)
		return string(data), nil
	}

	// 3. Coding Agent
	// If it looks like a coding task (contains "PRIMES" or "python")
	if strings.Contains(prompt, "PRIMES") || strings.Contains(strings.ToLower(prompt), "python") {
		return `
I will implement the primes calculation script.

` + "```python" + `
import json

def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

primes = [x for x in range(2, 100) if is_prime(x)]

with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)

print("Generated primes.json")
` + "```" + `
`, nil
	}

	// 4. QA Agent
	if strings.Contains(strings.ToLower(prompt), "qa agent") {
		return `
QA Check Passed.
The file primes.json exists and contains valid JSON.
QA_PASSED=true
`, nil
	}

	// 5. Manager Agent
	if strings.Contains(strings.ToLower(prompt), "manager agent") {
		// Ensure we mark features as passed
		return `
I have reviewed the work. It looks correct.
The primes.json file is present.

` + "```bash" + `
agent-bridge feature set "PRIMES" --status passed --passes
` + "```" + `

Project signed off.
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
	return s[:maxLen]
}
