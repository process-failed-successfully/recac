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

	// 1. TPM Agent Role (Ticket Generation)
	// Check if this is a request to generate tickets (usually contains this system prompt)
	if strings.Contains(prompt, "Technical Program Manager") {
		// Return a JSON ticket plan for the 'prime-python' scenario
		// The orchestrator smoke test looks for ID:[PRIMES]
		return `[
  {
    "title": "ID:[PRIMES] Prime Number Script",
    "description": "Implement a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to a file named 'primes.json'. The script must not use any external libraries for calculation.",
    "type": "Task",
    "children": [],
    "acceptance_criteria": [
      "primes.py exists",
      "primes.json exists and contains correct primes",
      "Correct number of primes (1229) are found"
    ]
  }
]`, nil
	}

	// 2. Coding Agent Role (Implementation)
	// Check if this is a request to implement the prime number script
	// The prompt will usually contain the ticket description we generated above
	if strings.Contains(prompt, "Prime Number Script") || strings.Contains(prompt, "ID:[PRIMES]") {
		// Return a bash script that implements the requirements
		// We use a bash block because the runner expects markdown with code blocks
		return "```bash\n" +
			"cat << 'EOF' > primes.py\n" +
			"import json\n" +
			"\n" +
			"def is_prime(n):\n" +
			"    if n < 2:\n" +
			"        return False\n" +
			"    for i in range(2, int(n**0.5) + 1):\n" +
			"        if n % i == 0:\n" +
			"            return False\n" +
			"    return True\n" +
			"\n" +
			"primes = [i for i in range(10000) if is_prime(i)]\n" +
			"print(f'Found {len(primes)} primes')\n" +
			"\n" +
			"with open('primes.json', 'w') as f:\n" +
			"    json.dump({'primes': primes}, f)\n" +
			"EOF\n" +
			"\n" +
			"# Run the script to generate output\n" +
			"python3 primes.py\n" +
			"\n" +
			"# Commit the results\n" +
			"git add primes.py primes.json\n" +
			"git commit -m \"Implement prime number calculation\"\n" +
			"git push\n" +
			"```", nil
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
