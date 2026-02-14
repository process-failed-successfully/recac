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

	// Heuristics for Smoke Test Compliance
	// 1. TPM / Jira Ticket Generation
	// The smoke test expects a single Task for ID:[PRIMES].
	// Returning multiple tickets or an Epic causes issues with the test runner and orchestrator filtering.
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "TPM") {
		return `[
  {
    "title": "ID:[PRIMES] Implement Prime Number Generator",
    "type": "Task",
    "description": "Implement a Python script to generate prime numbers.",
    "acceptance_criteria": [
      "Must generate primes up to 10000",
      "Must output to primes.json",
      "Must be efficient"
    ],
    "dependencies": []
  }
]`, nil
	}

	// 2. Coding / Script Generation (PRIMES)
	// This heuristic triggers when the agent picks up the ticket created above.
	// We broaden the check to catch "Prime Number" or "primes.json" in the prompt text
	// as "ID:[PRIMES]" might not always be present or fully formed in the agent prompt.
	isCodingPrompt := strings.Contains(prompt, "ID:[PRIMES]") ||
		strings.Contains(prompt, "primes.py") ||
		strings.Contains(prompt, "Prime Number") ||
		strings.Contains(prompt, "primes.json")

	if isCodingPrompt {
		return `#!/bin/bash
# Implement primes.py
cat <<EOF > primes.py
import json
import sys

def generate_primes(n):
    primes = []
    # Calculate primes LESS THAN n
    for i in range(2, n):
        is_prime = True
        for j in range(2, int(i ** 0.5) + 1):
            if i % j == 0:
                is_prime = False
                break
        if is_prime:
            primes.append(i)
    return primes

if __name__ == "__main__":
    limit = 10000
    if len(sys.argv) > 1:
        limit = int(sys.argv[1])

    primes = generate_primes(limit)

    # Output to file as required by acceptance criteria
    with open("primes.json", "w") as f:
        json.dump({"primes": primes}, f)

    print(f"Generated {len(primes)} primes")
EOF

# Ensure git configuration
git config user.email "you@example.com"
git config user.name "Your Name"

# Run the script to generate the json file immediately
python3 primes.py

# Commit and Push both files
git add primes.py primes.json
git commit -m "Implement primes.py and add generated primes.json"
git push origin HEAD
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
