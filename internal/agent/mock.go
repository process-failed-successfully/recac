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

	// Logic for "prime-python" scenario (Smoke Test)
	// 1. Feature Generation (Initializer Agent)
	if strings.Contains(prompt, "## YOUR ROLE - INITIALIZER AGENT") {
		return `[
  {
    "title": "Implement Prime Number Generator",
    "description": "Create a Python script that calculates prime numbers up to 10000 and saves them to primes.json.",
    "acceptance_criteria": [
      "Script is named primes.py",
      "Output file is primes.json",
      "Contains correct primes"
    ],
    "priority": "High",
    "id": "PRIMES",
    "files": ["primes.py"]
  }
]`, nil
	}

	// 2. Implementation (Developer Agent)
	// The prompt usually contains the ticket description or "ID:[PRIMES]" if we parse it.
	// Or we can look for specific keywords in the prompt.
	if strings.Contains(prompt, "Create a Python script that calculates prime numbers") ||
	   strings.Contains(prompt, "ID:[PRIMES]") {

		// If the prompt ALSO contains the file content (which means we already implemented it),
		// we should move to QA or finish.
		// The runner loop sends back the file content after execution.
		if strings.Contains(prompt, "def is_prime(n):") {
			// We already implemented it. Signal completion.
			return "TRIGGER_QA", nil
		}

		return `
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

primes = [i for i in range(10000) if is_prime(i)]

with open('primes.json', 'w') as f:
    json.dump({'primes': primes}, f)
EOF

# Run the script to generate the output file
python3 primes.py
`, nil
	}

	// 3. QA Agent
	if strings.Contains(prompt, "## YOUR ROLE - QA AGENT") {
		return "QA_PASSED", nil
	}

	// 4. Project Manager / Signoff
	if strings.Contains(prompt, "## YOUR ROLE - PROJECT MANAGER") {
		return "PROJECT_SIGNED_OFF", nil
	}

    // Default Fallback
	// Return a mock response that shows the agent received the prompt
	// This allows the session to run without requiring real API keys
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\n# no-op\n",
		m.responsePrefix, len(prompt))
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
