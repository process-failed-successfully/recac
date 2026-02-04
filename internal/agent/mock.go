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

	// Heuristics for E2E Smoke Tests

	// 1. Ticket Generation (TPM)
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "tickets") {
		// Return valid JSON for tickets
		// The E2E test expects a ticket with ID:[PRIMES] in the title to map to the scenario
		return `[
  {
    "title": "ID:[PRIMES] Implement Prime Number Script",
    "description": "Create a Python script that calculates prime numbers.",
    "type": "Task",
    "labels": ["recac-e2e", "backend"],
    "assignee": "recac-agent"
  }
]`, nil
	}

	// 2. Implementation (Developer) for Prime Python Scenario
	if strings.Contains(prompt, "[PRIMES]") || strings.Contains(prompt, "primes.py") {
		// Return a bash script that implements the requirement
		// The script writes a python file and optionally runs it
		return "```bash\n" +
			"cat << 'EOF' > primes.py\n" +
			"def is_prime(n):\n" +
			"    if n <= 1: return False\n" +
			"    for i in range(2, int(n**0.5) + 1):\n" +
			"        if n % i == 0: return False\n" +
			"    return True\n\n" +
			"primes = [i for i in range(2, 21) if is_prime(i)]\n" +
			"print(primes)\n" +
			"import json\n" +
			"with open('primes.json', 'w') as f:\n" +
			"    json.dump(primes, f)\n" +
			"EOF\n\n" +
			"python3 primes.py\n" +
			"```", nil
	}

	// 3. QA Agent
	if strings.Contains(prompt, "QA AGENT") {
		return "```bash\nagent-bridge signal QA_PASSED true\n```\nQA passed.", nil
	}

	// 4. Project Manager
	if strings.Contains(prompt, "PROJECT MANAGER") {
		return "```bash\nagent-bridge signal PROJECT_SIGNED_OFF true\n```\nProject signed off.", nil
	}

	// Default response
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
