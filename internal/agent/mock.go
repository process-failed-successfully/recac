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

	// Heuristics for E2E tests

	// Initializer Agent
	if strings.Contains(prompt, "You are an Initializer Agent") || strings.Contains(prompt, "INITIALIZER AGENT") {
		return `
I will generate the initial feature list.

` + "```bash" + `
cat <<EOF > feature_list.json
[
  {
    "id": "primes-script",
    "title": "Implement Primes Script",
    "description": "Create a Python script that generates prime numbers and writes them to primes.json",
    "type": "feature",
    "status": "todo"
  }
]
EOF

agent-bridge import --file feature_list.json --project "$RECAC_PROJECT_ID"
` + "```", nil
	}

	// TPM / Manager (Ticket Generation)
	if strings.Contains(prompt, "create JIRA tickets") || strings.Contains(prompt, "TPM Agent") {
		// If this is for ticket generation (CLI), return strictly formatted JSON
		if strings.Contains(prompt, "JSON list") {
			return "```json\n" + `[
  {
    "title": "ID:[PRIMES] Implement Prime Number Script",
    "description": "Create a Python script that calculates prime numbers.",
    "type": "Story",
    "acceptance_criteria": [
      "Script generates primes.json",
      "Script verified correct"
    ],
    "children": []
  }
]` + "\n```", nil
		}

		// If this is for the runner loop (Project Manager), signal sign-off
		return `
I will create the tickets.

` + "```bash" + `
# Signal project sign off
agent-bridge signal --type PROJECT_SIGNED_OFF --reason "Plan approved" --privileged
` + "```", nil
	}

	// Coding Agent (Primes)
	if strings.Contains(prompt, "YOUR ROLE - CODING AGENT") || strings.Contains(strings.ToLower(prompt), "prime") {
		return `
I will implement the primes script.

` + "```python" + `
import json

def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

primes = [x for x in range(2, 100) if is_prime(x)]
with open("primes.json", "w") as f:
    json.dump(primes, f)
` + "```" + `

` + "```bash" + `
# Mark task as completed
agent-bridge feature set primes-script --status completed --passes true
` + "```", nil
	}

	// QA / Review
	if strings.Contains(prompt, "QA") || strings.Contains(prompt, "REVIEW") || strings.Contains(prompt, "VERIFY") {
		return "LGTM", nil
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
