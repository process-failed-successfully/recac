package agent

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// MockAgent is a smart mock agent for testing and mock mode
// It returns context-aware responses to simulate E2E scenarios
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
	// The TPM prompt usually starts with specific role definition
	if strings.Contains(prompt, "You are an expert Technical Program Manager (TPM)") {
		// Return a JSON plan for the 'Primes' scenario wrapped in a bash script
		// This ensures the circuit breaker doesn't trip on "no-op" (empty execution output)
		return `#!/bin/bash
cat << 'EOF'
[
  {
    "title": "ID:[PRIMES] Implement Primes",
    "description": "Implement a Python script to calculate prime numbers.",
    "type": "Task",
    "acceptance_criteria": [
      "Must correctly identify prime numbers",
      "Must be a valid Python script"
    ],
    "dependencies": []
  }
]
EOF
`, nil
	}

	// 2. Initializer Agent
	// Responsible for importing the feature list into the database
	if strings.Contains(prompt, "## YOUR ROLE - INITIALIZER AGENT") {
		// Return a bash script that imports the features
		// We use a bash script because the agent executes the response
		return `#!/bin/bash
cat << 'EOF' > feature_list.json
[
  {
    "id": "PRIMES",
    "title": "ID:[PRIMES] Implement Primes",
    "description": "Implement a Python script to calculate prime numbers.",
    "type": "Task",
    "status": "todo"
  }
]
EOF
agent-bridge import feature_list.json
`, nil
	}

	// 3. Coding Agent
	// Detects if we are assigned to write code
	// Heuristics: Header check OR content keywords for the specific task
	isCodingAgent := strings.Contains(prompt, "## YOUR ROLE - CODING AGENT")
	isPrimesTask := strings.Contains(prompt, "[PRIMES]") || strings.Contains(prompt, "primes.py") || strings.Contains(prompt, "Prime Number Script")

	if isCodingAgent || isPrimesTask {
		// Return a bash script that:
		// 1. Creates the python file
		// 2. Runs it to verify
		// 3. Updates the feature status
		// 4. Signals QA passed (skipping explicit QA agent for simplicity in smoke test if not requested)

		script := `#!/bin/bash
# Implement primes.py
cat << 'EOF' > primes.py
def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

if __name__ == "__main__":
    import sys
    # Print primes up to 20
    for i in range(20):
        if is_prime(i):
            print(f"{i} is prime")
EOF

# Run it to verify
python3 primes.py

# Update feature status (ID is inferred as PRIMES)
agent-bridge feature update PRIMES --status completed

# Signal completion for this agent loop
agent-bridge signal QA_PASSED true
`
		return script, nil
	}

	// 4. QA / Manager / Reviewer
	// Detects requests for sign-off or review
	if strings.Contains(prompt, "## YOUR ROLE - QA AGENT") ||
	   strings.Contains(prompt, "## YOUR ROLE - PROJECT MANAGER") ||
	   strings.Contains(prompt, "review") {

		// If it's a review/manager step, we assume success for the smoke test
		// and signal the project is done.
		return `agent-bridge signal PROJECT_SIGNED_OFF true --privileged || touch PROJECT_SIGNED_OFF`, nil
	}

	// Default Fallback
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100))
	return response, nil
}

// SendStream implements the Agent interface
func (m *MockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	// Simulate a small delay to mimic streaming, but keep it short for tests
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
