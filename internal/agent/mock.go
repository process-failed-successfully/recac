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

	// Heuristic: Ticket Generation for Prime Python Scenario
	if strings.Contains(prompt, "ID:[PRIMES]") && strings.Contains(prompt, "Create a SINGLE Ticket") {
		return `[
  {
    "title": "ID:[PRIMES] Prime Number Script",
    "description": "Implement a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to a file named 'primes.json'. The JSON format must have a single key 'primes' containing the list of integers. Example: {\"primes\": [2, 3, 5, ...]}. The script MUST be named 'primes.py'. The output file MUST be named 'primes.json'. IMPORTANT: You MUST use a bash block to create the file. Commit 'primes.json' IMMEDIATELY after creating/running the script. Do NOT leave it untracked.",
    "type": "Task",
    "acceptance_criteria": [
      "primes.py calculates 1229 primes",
      "primes.json contains the list of primes"
    ]
  }
]`, nil
	}

	// Heuristic: Task Execution for Prime Python Scenario
	// We look for the task description in the prompt. We use a more relaxed match to handle variations.
	if strings.Contains(prompt, "primes.py") && (strings.Contains(prompt, "python") || strings.Contains(prompt, "ID:[PRIMES]")) {
		// Return bash script to do the work and SIGNAL COMPLETION
		return `I will create the python script to calculate primes.

` + "```bash" + `
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
` + "```" + `

Now I will run the script, commit the results, and signal completion.

` + "```bash" + `
python3 primes.py
git add primes.py primes.json
git commit -m "Add primes script and output" --allow-empty
agent-bridge signal PROJECT_SIGNED_OFF true
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
