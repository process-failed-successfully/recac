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

	// Check for 'Planning' (TPM) scenario trigger
	// Must come BEFORE Primes check because the planning prompt contains the spec with "ID:[PRIMES]"
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "TPM") {
		return m.handlePlanningScenario(), nil
	}

	// Check for 'Primes' E2E scenario trigger
	if strings.Contains(prompt, "ID:[PRIMES]") || strings.Contains(prompt, "primes.py") {
		return m.handlePrimesScenario(), nil
	}

	// Return a mock response that shows the agent received the prompt
	// This allows the session to run without requiring real API keys
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100))
	return response, nil
}

func (m *MockAgent) handlePrimesScenario() string {
	// The Python script to generate primes
	scriptContent := `
import json

def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = [x for x in range(10000) if is_prime(x)]

with open("primes.json", "w") as f:
    json.dump({"primes": primes}, f)
`
	// Return a bash block that creates the file, runs it, commits, and signals completion
	return fmt.Sprintf(`I will implement the prime number script as requested.

` + "```bash" + `
# Create the python script
cat << 'EOF' > primes.py
%s
EOF

# Run the script to generate primes.json
python3 primes.py

# Commit the results (force add just in case)
git add -f primes.py primes.json
git commit -m "Add primes script and output" || echo "Nothing to commit"

# Signal completion to stop the agent loop
# The agent-bridge CLI is used to communicate with the orchestrator
agent-bridge signal --privileged PROJECT_SIGNED_OFF
` + "```" + `
`, scriptContent)
}

func (m *MockAgent) handlePlanningScenario() string {
	// Return a JSON array of tickets for the Planning Agent
	// The recac generate-from-spec command expects strict JSON
	return `[
  {
    "id": "PRIMES",
    "summary": "Create Prime Number Script",
    "description": "Implement a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to a file named 'primes.json'.",
    "type": "Task"
  }
]`
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
