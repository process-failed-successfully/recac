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

	// TPM: Generate Tickets
	// Detects the prompt asking for ticket creation (usually contains "Technical Program Manager" and "Ticket")
	if strings.Contains(prompt, "Technical Program Manager") && (strings.Contains(prompt, "Ticket") || strings.Contains(prompt, "tickets")) {
		return `[
  {
    "id": "PRIMES",
    "title": "ID:[PRIMES] Prime Number Script",
    "description": "Implement a python script named 'primes.py' that calculates all prime numbers less than 10,000.",
    "type": "Task"
  }
]`, nil
	}

	// Developer: Implement primes.py
	// Detects the prompt asking for the primes script
	if strings.Contains(prompt, "primes.py") && !strings.Contains(prompt, "Technical Program Manager") {
		// If primes.json already exists (likely in the file list or git log in the prompt), we are done.
		if strings.Contains(prompt, "primes.json") {
			return "```bash\nagent-bridge feature set --status done\n```", nil
		}

		return "```bash\n" +
			"git config user.email \"agent@recac.com\"\n" +
			"git config user.name \"Recac Agent\"\n" +
			"cat << 'EOF' > primes.py\n" +
			"import json\n" +
			"\n" +
			"def get_primes(n):\n" +
			"    primes = []\n" +
			"    for num in range(2, n):\n" +
			"        is_prime = True\n" +
			"        for i in range(2, int(num ** 0.5) + 1):\n" +
			"            if num % i == 0:\n" +
			"                is_prime = False\n" +
			"                break\n" +
			"        if is_prime:\n" +
			"            primes.append(num)\n" +
			"    return primes\n" +
			"\n" +
			"primes = get_primes(10000)\n" +
			"with open('primes.json', 'w') as f:\n" +
			"    json.dump({\"primes\": primes}, f)\n" +
			"EOF\n" +
			"\n" +
			"python3 primes.py\n" +
			"git add primes.py primes.json\n" +
			"git diff --quiet && git diff --staged --quiet || git commit -m \"Implement primes.py and generate primes.json\" --author=\"Recac Agent <agent@recac.com>\" --no-gpg-sign\n" +
			"```", nil
	}

	// QA Agent: Approve
	if strings.Contains(prompt, "QA AGENT") {
		return "```bash\nagent-bridge signal QA_PASSED true\n```", nil
	}

	// Project Manager: Sign off
	if strings.Contains(prompt, "PROJECT MANAGER") {
		return "```bash\nagent-bridge signal PROJECT_SIGNED_OFF true\n```", nil
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
