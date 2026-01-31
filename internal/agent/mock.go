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

	// Heuristics for Mock Behavior in CI/Tests

	// 1. TPM Agent (Ticket Generation)
	// The prompt usually comes from prompts.TPMAgent and contains the spec.
	// We check for keywords like "ticket", "JSON array" and specific scenario markers like "PRIMES".
	// The "PRIMES" check ensures we only return this specific JSON for the Prime Python scenario.
	if (strings.Contains(prompt, "ticket") || strings.Contains(prompt, "Ticket")) && strings.Contains(prompt, "PRIMES") {
		// Return a valid JSON list of tickets for the Prime Python scenario
		// Note: The scenario expects ID [PRIMES] mapping to a Task.
		jsonResponse := `[
  {
    "title": "ID:[PRIMES] Implement Prime Number Script",
    "description": "Implement a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to a file named 'primes.json'.",
    "type": "Task",
    "acceptance_criteria": [
      "primes.py created",
      "primes.json generated with correct primes",
      "Files committed"
    ],
    "children": []
  }
]`
		return jsonResponse, nil
	}

	// 2. Worker Agent (Task Execution) - Prime Python Scenario
	// The worker prompt asks to implement the task described in the ticket.
	// For Prime Python scenario, it asks for 'primes.py'.
	if strings.Contains(prompt, "primes.py") {
		// Return a bash script to create the python script and run it
		// We use a bash block as expected by the executor
		bashResponse := `
I will create the primes.py script and generate the json file.

` + "```bash" + `
# Create primes.py
cat << 'EOF' > primes.py
import json

def get_primes(n):
    primes = []
    for num in range(2, n):
        is_prime = True
        for i in range(2, int(num ** 0.5) + 1):
            if num % i == 0:
                is_prime = False
                break
        if is_prime:
            primes.append(num)
    return primes

primes = get_primes(10000)
with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
EOF

# Run the script
python3 primes.py

# Commit files
# Note: In local mode, git identity might be needed if not set globally
git config user.email "bot@recac.local"
git config user.name "Recac Bot"
git add primes.py primes.json
git commit -m "Add primes.py and primes.json"
` + "```" + `
`
		return bashResponse, nil
	}

	// Default response for other cases
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
