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

	// Smart Mocking for Smoke Tests

	// 1. Ticket Generation / Planning Phase
	// Detects the AppSpec header for the Primes scenario
	if strings.Contains(prompt, "ID:[PRIMES] Prime Number Script") && strings.Contains(prompt, "AppSpec") {
		return `[
  {
    "title": "ID:[PRIMES] Create Prime Number Script",
    "description": "Create a python script named 'primes.py'. It MUST be python.\nIt must calculate all prime numbers less than 10,000 and output to a file named 'primes.json'.\nIMPORTANT: You MUST use a bash block to create the file (e.g., cat << 'EOF' > primes.py). Do not output raw python code.\nCommit 'primes.py' and 'primes.json' IMMEDIATELY. Use 'git add -f primes.json' to ensure it is tracked.\nThe JSON format must have a single key 'primes' containing the list of integers.\nExample: {\"primes\": [2, 3, 5, ...]}.\nIMPORTANT: Ensure the FINAL primes.json committed to the repository contains ALL primes less than 10,000 (Exactly 1229 primes).\nDo not truncate it for testing or reporting - the verification script expects the full list.\nKeep the code absolutely minimal. Finish as quickly as possible.\n\nCRITICAL: You MUST name the script 'primes.py'. Do not use 'feature_implementation.py' or any other generic name.\nCRITICAL: Do NOT run 'pytest' or any test framework. Do NOT try to create test files. Just run the script and verify 'primes.json' exists.",
    "type": "Task",
    "children": []
  }
]`, nil
	}

	// 2. Implementation Phase
	// Detects the specific instruction from the ticket description
	if strings.Contains(prompt, "Create a python script named 'primes.py'") {
		return `I will create the 'primes.py' script to calculate prime numbers and output them to 'primes.json', then commit the changes.

` + "```bash" + `
# Create primes.py
cat << 'EOF' > primes.py
import json

def get_primes(n):
    primes = []
    for i in range(2, n):
        is_prime = True
        for j in range(2, int(i ** 0.5) + 1):
            if i % j == 0:
                is_prime = False
                break
        if is_prime:
            primes.append(i)
    return primes

primes = get_primes(10000)
with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
EOF

# Run the script to generate primes.json
python3 primes.py

# Commit changes
git add primes.py primes.json
git commit -m "Add primes.py and generated primes.json"
` + "```" + `
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
