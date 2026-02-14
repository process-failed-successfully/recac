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

	// Heuristic 1: Technical Program Manager (Planning Phase)
	if strings.Contains(prompt, "Technical Program Manager (TPM)") || strings.Contains(prompt, "agile software development") {
		return `[
  {
    "id": "ID:[PRIMES]",
    "title": "Implement Prime Number Generator",
    "description": "Create a python script primes.py that generates the first 100 prime numbers and writes them to primes.json",
    "repository_url": "https://github.com/process-failed-successfully/recac-jira-e2e",
    "children": [
      {
        "id": "req-script-runs-without-errors",
        "title": "Script runs without errors",
        "description": "Script runs without errors",
        "type": "requirement"
      },
      {
        "id": "req-output-file-primes-json-contai",
        "title": "Output file primes.json contains valid JSON array of primes",
        "description": "Output file primes.json contains valid JSON array of primes",
        "type": "requirement"
      }
    ]
  }
]`, nil
	}

	// Heuristic 2: Coding Phase (Primes Implementation)
	// Triggers on "ID:[PRIMES]", "primes.py", or "1229"
	if strings.Contains(prompt, "ID:[PRIMES]") || strings.Contains(prompt, "primes.py") || strings.Contains(prompt, "1229") {
		// Check if we need to detect completion (idempotency)
		// Usually the prompt includes the git status output from the previous iteration
		if strings.Contains(prompt, "nothing to commit") || strings.Contains(prompt, "working tree clean") {
			return `I see that the work is already done and committed. I will mark the project as complete.

` + "```bash" + `
agent-bridge signal PROJECT_SIGNED_OFF true --privileged
` + "```", nil
		}

		// Otherwise, provide the implementation
		return `I will implement the prime number generator script as requested.

Here is the plan:
1. Create ` + "`primes.py`" + `
2. Run it to generate ` + "`primes.json`" + `
3. Verify requirements
4. Commit and sign off

` + "```bash" + `
# Create the python script
cat <<EOF > primes.py
import json

def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

primes = []
num = 2
while len(primes) < 100:
    if is_prime(num):
        primes.append(num)
    num += 1

with open('primes.json', 'w') as f:
    json.dump(primes, f)

print(f"Generated {len(primes)} primes")
EOF

# Execute the script to generate the artifact
python3 primes.py

# Import requirements (simulated based on spec)
agent-bridge import --id req-script-runs-without-errors --description "Script runs without errors"
agent-bridge import --id req-output-file-primes-json-contai --description "Output file primes.json contains valid JSON array of primes"

# Mark requirements as passed
agent-bridge feature set req-script-runs-without-errors completed
agent-bridge feature set req-output-file-primes-json-contai completed

# Signal completion
agent-bridge signal PROJECT_SIGNED_OFF true --privileged
` + "```", nil
	}

	// Default fallback response
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
