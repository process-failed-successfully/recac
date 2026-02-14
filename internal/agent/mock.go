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
// It implements heuristics to support E2E smoke tests.
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// Heuristic 1: Jira Ticket Generation (TPM Agent)
	// Triggers if prompt contains "ID:[PRIMES]" (from spec) or "Ticket" creation instructions.
	// The prompt typically contains "ID:[PRIMES]" when used in prime-python scenario.
	if strings.Contains(prompt, "ID:[PRIMES]") && (strings.Contains(prompt, "ticket") || strings.Contains(prompt, "Task")) {
		// Return JSON ticket list for prime-python scenario
		// Note: We omit "Repo:" from description so the CLI can inject the correct E2E repo URL.
		return `[
  {
    "title": "ID:[PRIMES] Create Prime Number Script",
    "description": "Implement a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to a file named 'primes.json'.",
    "type": "Task",
    "acceptance_criteria": [
      "Output file primes.json exists",
      "Contains correct primes"
    ]
  }
]`, nil
	}

	// Heuristic 2: Coding Agent (Primes Implementation)
	// Triggers if prompt asks to implement 'primes.py' or contains "ID:[PRIMES]" in a coding context.
	// We also check for "primes.json" or "prime" to catch variations where feature description only mentions requirements.
	// We specifically look for "CODING AGENT" role header to avoid misfiring on other agents.
	isCodingAgent := strings.Contains(prompt, "YOUR ROLE - CODING AGENT")
	hasPrimesContext := strings.Contains(prompt, "primes.py") || strings.Contains(prompt, "primes.json") || strings.Contains(prompt, "ID:[PRIMES]") || strings.Contains(strings.ToLower(prompt), "prime number")

	// Fallback for generic "Continue implementing" prompt when no specific task is selected yet
	isGenericTask := strings.Contains(prompt, "Continue implementing pending features")

	if isCodingAgent && (hasPrimesContext || isGenericTask) {
		// Return bash script to create the files
		// We pre-calculate primes to ensure correctness without running python in the mock response generator.
		// We also explicitly signal completion via agent-bridge feature set.

		// Note: The prompt might be for a specific feature (e.g. "Output file primes.json exists").
		// We should just do the whole task in one go if possible, or at least ensure the files exist.

		return `I will implement the prime number script features.

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

# Run the script to generate the json
python3 primes.py

# Verify output
ls -l primes.json

# Commit the files
git add primes.py primes.json
git commit -m "Add primes.py and primes.json" || echo "Nothing to commit"

# Mark features as done (Blindly mark all related features)
# We guess the IDs might be related to requirements
agent-bridge feature set req-output-file-primes-json-exists --status done --passes true || true
agent-bridge feature set req-contains-correct-primes --status done --passes true || true

` + "```" + `
`, nil
	}

	// Default Mock Response
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
