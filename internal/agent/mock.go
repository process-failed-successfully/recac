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
	project        string
	model          string
}

// NewMockAgent creates a new mock agent
func NewMockAgent(apiKey, model, project string) *MockAgent {
	return &MockAgent{
		responsePrefix: "Mock agent response",
		project:        project,
		model:          model,
	}
}

// SetResponse forces a specific response from the agent
func (m *MockAgent) SetResponse(response string) {
	m.forcedResponse = response
}

// Send implements the Agent interface
// It returns a mock response that acknowledges the prompt
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	// DEBUG: Print prompt to stderr to debug CI failures where heuristics fail
	fmt.Fprintf(os.Stderr, "[MockAgent] Received prompt (%d chars): %s\n", len(prompt), truncateString(prompt, 200))

	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	promptUpper := strings.ToUpper(prompt)

	// Technical Program Manager Heuristic (JSON Tickets)
	// Must return a ticket with ID "PRIMES" to align with smoke test job expectations
	if strings.Contains(promptUpper, "TECHNICAL PROGRAM MANAGER") && (strings.Contains(promptUpper, "TICKET") || strings.Contains(promptUpper, "PLAN")) {
		return `[
  {
    "title": "Implement Prime Number Script",
    "description": "Implement a Python script that calculates primes up to 10,000.",
    "type": "Task",
    "id": "PRIMES"
  }
]`, nil
	}

	// Loop Breaker / Feature Completion Heuristic
	if strings.Contains(promptUpper, "NOTHING TO COMMIT") || strings.Contains(promptUpper, "WORKING TREE CLEAN") {
		return "The feature is complete and verified.\n```bash\nagent-bridge feature set req-primes-implementation --status done --passes true\n```", nil
	}

	// Coding Agent Heuristic (Primes Scenario)
	// Must calculate primes up to 10,000 to satisfy E2E verification
	// Also check feature ID specifically
	if strings.Contains(promptUpper, "CODING AGENT") && (strings.Contains(promptUpper, "PRIMES") || strings.Contains(promptUpper, "REQ-PRIMES-IMPLEMENTATION")) {
		return "I will implement the primes script.\n```python\ndef primes(n):\n    primes = []\n    for i in range(2, n + 1):\n        is_prime = True\n        for j in range(2, int(i ** 0.5) + 1):\n            if i % j == 0:\n                is_prime = False\n                break\n        if is_prime:\n            primes.append(i)\n    return primes\n\nif __name__ == '__main__':\n    import json\n    print(json.dumps(primes(10000)))\n```", nil
	}

	// Initializer Agent Heuristic
	// Must return a valid FeatureList JSON with repository_url matching the E2E test expectation
	// Broadened heuristics to catch variations in prompt template
	if strings.Contains(promptUpper, "INITIALIZER") ||
	   strings.Contains(promptUpper, "GET YOUR BEARINGS") ||
	   strings.Contains(promptUpper, "FIRST AGENT") ||
	   strings.Contains(promptUpper, "CREATE FEATURE_LIST.JSON") ||
	   strings.Contains(promptUpper, "YOUR ROLE") {
		return `I will initialize the feature list.
` + "```bash" + `
cat <<EOF > feature_list.json
{
  "project_name": "recac-jira-e2e",
  "repository_url": "https://github.com/process-failed-successfully/recac-jira-e2e",
  "features": [
    {
      "id": "req-primes-implementation",
      "category": "core",
      "priority": "MVP",
      "description": "Implement a Python script to calculate prime numbers.",
      "status": "todo",
      "passes": false,
      "steps": [],
      "dependencies": {
        "depends_on_ids": [],
        "exclusive_write_paths": [],
        "read_only_paths": []
      }
    }
  ]
}
EOF
agent-bridge import < feature_list.json
` + "```", nil
	}

	// QA Agent Heuristic
	if strings.Contains(promptUpper, "QA AGENT") {
		return "I have verified the changes.\n```bash\nagent-bridge signal QA_PASSED true\n```", nil
	}

	// Project Manager Heuristic
	if strings.Contains(promptUpper, "PROJECT MANAGER") {
		return "I approve the project.\n```bash\nagent-bridge signal PROJECT_SIGNED_OFF true --privileged\n```", nil
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
