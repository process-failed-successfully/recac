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
// It returns a mock response based on heuristics or forced response
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	upperPrompt := strings.ToUpper(prompt)

	// Heuristic 1: TPM Agent (Scenario Generation)
	// Matches prompt for TPM creating tickets for PRIMES scenario
	// The prompt usually contains "You are an expert Technical Program Manager" and scenario ID "[PRIMES]"
	if (strings.Contains(upperPrompt, "TPM") || strings.Contains(upperPrompt, "TECHNICAL PROGRAM MANAGER")) &&
		(strings.Contains(upperPrompt, "PRIMES") || strings.Contains(upperPrompt, "PRIME NUMBER")) {
		return `[
  {
    "title": "ID:[PRIMES] Implement Prime Number Generator",
    "description": "Create a Python script that generates prime numbers up to N. The script should be efficient and well-documented.\nRepo: https://github.com/process-failed-successfully/recac-jira-e2e",
    "type": "Task",
    "acceptance_criteria": [
      "Script accepts N as input",
      "Outputs JSON list of primes",
      "Includes unit tests"
    ],
    "children": []
  }
]`, nil
	}

	// Heuristic 2: Initializer Agent
	// Initialize features in the database. Prompt contains "INITIALIZER AGENT".
	if strings.Contains(upperPrompt, "INITIALIZER") {
		return "```bash\n" +
			"echo '[{\"id\": \"PRIMES\", \"name\": \"Prime Generator\", \"type\": \"Task\"}]' | agent-bridge import --project \"$RECAC_PROJECT_ID\"\n" +
			"```", nil
	}

	// Heuristic 3: Coding Agent (Implementation)
	// Matches prompt for "primes.py" or "prime number" and "python"/"script"
	if (strings.Contains(prompt, "primes.py") || strings.Contains(upperPrompt, "PRIME NUMBER")) && (strings.Contains(upperPrompt, "PYTHON") || strings.Contains(upperPrompt, "SCRIPT")) {
		return "```bash\n" +
			"cat << 'EOF' > primes.py\n" +
			"import json\n" +
			"import sys\n" +
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
			"\n" +
			"with open('primes.json', 'w') as f:\n" +
			"    json.dump({'primes': primes}, f)\n" +
			"EOF\n" +
			"\n" +
			"python3 primes.py\n" +
			"git add primes.py primes.json\n" +
			"git commit -m \"Implement prime generator\"\n" +
			"agent-bridge feature set PRIMES --status completed --passes true\n" +
			"```", nil
	}

	// Heuristic 4: QA/Review
	if strings.Contains(upperPrompt, "REVIEW") || strings.Contains(upperPrompt, "VERIFY") {
		return "LGTM", nil
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
