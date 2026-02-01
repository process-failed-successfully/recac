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

	// 1. Planner Prompt (Autonomous Agent Loop)
	// Expects: { "features": [...] }
	if strings.Contains(prompt, "Create a JSON object containing a feature list") {
		return `
{
  "features": [
    {
      "id": "[PRIMES]",
      "description": "Implement prime number calculation script",
      "type": "Task"
    }
  ]
}`, nil
	}

	// 2. TPM Agent Prompt (CLI: recac jira generate-from-spec)
	// Expects: [ { "title": "...", "children": [...] } ]
	if strings.Contains(prompt, "You are an expert Technical Program Manager (TPM)") {
		// Extract repo URL if present in prompt to include in description, ensuring validation passes
		// (though usually injected by the caller, we mock it here)
		return `
[
  {
    "title": "ID:[PRIMES] Prime Number Implementation",
    "description": "Implement the prime number generation script. Repo: https://github.com/process-failed-successfully/recac-jira-e2e",
    "type": "Epic",
    "children": [
      {
        "title": "ID:[PRIMES] Create primes.py",
        "description": "Create a python script named 'primes.py' that calculates primes < 10000. Repo: https://github.com/process-failed-successfully/recac-jira-e2e",
        "type": "Task",
        "acceptance_criteria": [
          "Script is named primes.py",
          "Output is named primes.json"
        ]
      }
    ]
  }
]`, nil
	}

	// 3. Initializer Prompt (Agent Bridge Import)
	// Triggered when feature list is missing. We MUST import features to stop the loop.
	if strings.Contains(prompt, "agent-bridge import") {
		return `I will import the features as requested.

` + "```bash" + `
cat << 'EOF' | agent-bridge import
{
  "features": [
    {
      "id": "[PRIMES]",
      "description": "Implement prime number calculation script",
      "type": "Task"
    }
  ]
}
EOF
` + "```" + `
`, nil
	}

	// 4. Implementation Prompt (Coding Agent)
	// Triggered if it asks for primes.py implementation details
	if strings.Contains(prompt, "primes.py") {
		// LOOP BREAK: If we see "nothing to commit" in the prompt (which includes history),
		// it means we already implemented it. Mark as done to break the loop.
		if strings.Contains(prompt, "nothing to commit") || strings.Contains(prompt, "working tree clean") || strings.Contains(prompt, "Nothing to commit") {
			return `It looks like the work is already done and committed. I will mark the task as completed.

` + "```bash" + `
agent-bridge update --id "[PRIMES]" --status done
echo "Task marked as done."
` + "```" + `
`, nil
		}

		return `I will create the primes.py script and the output file.

` + "```bash" + `
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n < 2: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = [x for x in range(10000) if is_prime(x)]

with open('primes.json', 'w') as f:
    json.dump({'primes': primes}, f)
EOF

python3 primes.py
git add primes.py primes.json
git commit -m "Implement primes script"
` + "```" + `
`, nil
	}

	// Default response with no-op bash block to prevent circuit breaker trip
	format := "%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...\n\n```bash\n# no-op\necho 'mock agent alive'\n```"
	response := fmt.Sprintf(format,
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
