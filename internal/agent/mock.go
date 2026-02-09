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
	var response string

	// Technical Program Manager Heuristic (JSON Tickets)
	// Must return a ticket with ID "PRIMES" to align with smoke test job expectations
	if strings.Contains(promptUpper, "TECHNICAL PROGRAM MANAGER") && (strings.Contains(promptUpper, "TICKET") || strings.Contains(promptUpper, "PLAN")) {
		response = `[
  {
    "title": "Implement Prime Number Script",
    "description": "Implement a Python script that calculates primes up to 10,000.",
    "type": "Task",
    "id": "PRIMES"
  }
]`
	} else if strings.Contains(promptUpper, "NOTHING TO COMMIT") || strings.Contains(promptUpper, "WORKING TREE CLEAN") {
		// Loop Breaker / Feature Completion Heuristic
		response = "The feature is complete and verified.\n```bash\nagent-bridge feature set req-primes-implementation --status done --passes true\n```"
	} else if strings.Contains(promptUpper, "INITIALIZER") ||
		strings.Contains(promptUpper, "GET YOUR BEARINGS") ||
		strings.Contains(promptUpper, "FIRST AGENT") ||
		strings.Contains(promptUpper, "CREATE FEATURE_LIST.JSON") {
		// Initializer Agent Heuristic
		// Must return a valid FeatureList JSON with repository_url matching the E2E test expectation
		// Broadened heuristics to catch variations in prompt template
		// Using explicit newlines for bash block robustness
		response = `I will initialize the feature list.
` + "```bash\n" + `cat <<EOF > feature_list.json
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
` + "\n```"
	} else if strings.Contains(promptUpper, "CODING AGENT") && (strings.Contains(promptUpper, "PRIMES") || strings.Contains(promptUpper, "REQ-PRIMES-IMPLEMENTATION")) {
		// Coding Agent Heuristic (Primes Scenario)
		// Must calculate primes up to 10,000 to satisfy E2E verification
		// CRITICAL: Must be a BASH block to be executed by the runner!
		response = `I will implement the primes script.
` + "```bash\n" + `cat <<EOF > primes.py
def primes(n):
    primes = []
    for i in range(2, n + 1):
        is_prime = True
        for j in range(2, int(i ** 0.5) + 1):
            if i % j == 0:
                is_prime = False
                break
        if is_prime:
            primes.append(i)
    return primes

if __name__ == '__main__':
    import json
    print(json.dumps(primes(10000)))
EOF

# Execute the script
python3 primes.py > primes.json

# Commit changes
git add primes.py primes.json
git commit -m "Implement primes script" || echo "Nothing to commit"

# Signal completion
agent-bridge feature set req-primes-implementation --status done --passes true
` + "\n```"
	} else if strings.Contains(promptUpper, "QA AGENT") {
		// QA Agent Heuristic
		response = "I have verified the changes.\n```bash\nagent-bridge signal QA_PASSED true\n```"
	} else if strings.Contains(promptUpper, "PROJECT MANAGER") {
		// Project Manager Heuristic
		response = "I approve the project.\n```bash\nagent-bridge signal PROJECT_SIGNED_OFF true --privileged\n```"
	} else {
		// Default response
		response = fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
			m.responsePrefix, len(prompt), truncateString(prompt, 100))
	}

	// DEBUG: Log the full response to help diagnose regex/executor issues
	fmt.Fprintf(os.Stderr, "[MockAgent] Sending response (%d chars):\n%s\n", len(response), response)

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
