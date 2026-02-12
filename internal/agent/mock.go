package agent

import (
	"context"
	"fmt"
	"regexp"
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

	// Heuristic for TPM / Jira Ticket Generation
	// Matches prompts asking for ticket creation or identifying as TPM
	if strings.Contains(prompt, "Technical Program Manager (TPM)") || strings.Contains(prompt, "Create exactly ONE ticket") {
		// Extract Repo URL if present to ensure tickets point to correct repo
		repoURL := "https://github.com/process-failed-successfully/recac-jira-e2e" // Default
		re := regexp.MustCompile(`Repo: (https?://[^\s]+)`)
		matches := re.FindStringSubmatch(prompt)
		if len(matches) > 1 {
			repoURL = matches[1]
		}

		// Return a JSON structure for the ticket plan
		// Must include "title" key for recac parser
		// Must include "ID:[PRIMES]" in child story title for E2E test lookup
		response := fmt.Sprintf(`[
  {
    "key": "MFLP-1",
    "title": "Implement Primes Calculation Feature",
    "description": "Create a python script to calculate primes.",
    "type": "Epic",
    "status": "To Do",
    "priority": "High",
    "children": [
      {
        "key": "MFLP-2",
        "title": "Create primes.py script ID:[PRIMES]",
        "description": "Write a python script that calculates primes up to 10000. The script should be named primes.py. Repo: %s",
        "type": "Story",
        "status": "To Do",
        "priority": "High"
      }
    ]
  }
]`, repoURL)
		return response, nil
	}

	// Heuristic for Initializer Agent
	if strings.Contains(strings.ToLower(prompt), "initializer agent") {
		// Return Bash script piping JSON to agent-bridge import
		// JSON structure: {"features": [...]}
		// The bridge expects a structured object, not a raw array
		jsonContent := `{"features": [{"id": "PRIMES", "description": "Calculate primes", "files": ["primes.py"]}]}`
		response := fmt.Sprintf("```bash\necho '%s' | agent-bridge import --file -\n```", jsonContent)
		return response, nil
	}

	// Heuristic for Coding Agent (primes.py)
	if strings.Contains(prompt, "primes.py") || strings.Contains(prompt, "CODING AGENT") {
		// Return Python script for primes
		// Script must range(10000) to match E2E assertion
		// Script must git add, git commit
		// Script must run agent-bridge feature set to verify feature
		response := "Here is the python script:\n\n```python\n" +
			"import os\n" +
			"def is_prime(n):\n" +
			"    if n <= 1: return False\n" +
			"    for i in range(2, int(n**0.5) + 1):\n" +
			"        if n % i == 0: return False\n" +
			"    return True\n\n" +
			"primes = [i for i in range(10000) if is_prime(i)]\n" +
			"print(f'Found {len(primes)} primes')\n" +
			"with open('primes.py', 'w') as f:\n" +
			"    f.write('print(\"Calculating primes...\")\\n')\n" +
			"```\n\n" +
			"And here is the bash script to commit and verify:\n\n```bash\n" +
			"git add primes.py\n" +
			"git commit -m 'feat: add primes calculation script'\n" +
			"agent-bridge feature set PRIMES --status completed --passes true\n" +
			"```"
		return response, nil
	}

	// Heuristic for QA Agent / Manager
	if strings.Contains(prompt, "Approve or Reject") || strings.Contains(prompt, "QA Agent") {
		return "```bash\nagent-bridge signal QA_PASSED true --privileged\n```", nil
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
