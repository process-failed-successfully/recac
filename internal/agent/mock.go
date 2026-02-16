package agent

import (
	"context"
	"fmt"
	"os"
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

	// Heuristic 1: Coding Agent - Execution Phase (Primes)
	// Check for "CODING AGENT" role or "primes" related content to return code
	// Prioritize this if we are in the coding loop to prevent returning JSON plans
	if (strings.Contains(prompt, "prime") || strings.Contains(prompt, "primes.json") || strings.Contains(strings.ToLower(prompt), "id:[primes]")) &&
		(strings.Contains(prompt, "CODING AGENT") || strings.Contains(prompt, "implementation")) {

		// Check if primes.json exists to determine if we should finish
		if _, err := os.Stat("primes.json"); err == nil {
			return `I have verified that primes.json exists. The task is complete.
DONE`, nil
		}

		return `
Here is the script to calculate primes.

` + "```bash" + `
cat <<EOF > primes.py
import json

def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

primes = [x for x in range(1, 10001) if is_prime(x)]
with open("primes.json", "w") as f:
    json.dump(primes, f)
EOF

python3 primes.py
` + "```" + `
`, nil
	}

	// Heuristic 2: Technical Program Manager - Planning Phase
	// This matches the TPM prompt template
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "TPM") || strings.Contains(prompt, "ticket generation") {
		// Return a valid JSON ticket plan as expected by the recac CLI
		// The key 'title' is critical for the CLI to parse it correctly
		return `[
  {
    "title": "ID:[PRIMES] Implement Prime Number Generator",
    "description": "Create a python script that generates prime numbers up to 10,000 and saves them to primes.json",
    "type": "Task",
    "status": "TODO",
    "priority": "High",
    "dependencies": []
  }
]`, nil
	}

	// Heuristic 3: QA Agent - Verification Phase
	if strings.Contains(prompt, "QA") || strings.Contains(prompt, "Quality Assurance") {
		return `The implementation appears correct. The primes.json file is generated.
DONE`, nil
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
