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

	// Heuristics for E2E Smoke Tests (Prime Python Scenario)
	lowerPrompt := strings.ToLower(prompt)

	// 1. Initializer Role: Create feature_list.json
	if strings.Contains(lowerPrompt, "create feature_list.json") || strings.Contains(lowerPrompt, "feature_list.json") {
		return `I will create the feature list.

` + "```bash" + `
cat << 'EOF' > feature_list.json
{
  "project_name": "ID:[PRIMES] Prime Number Script",
  "features": [
    {
      "id": "req-primes-py",
      "category": "functional",
      "priority": "critical",
      "description": "Implement primes.py to calculate primes < 10000",
      "status": "todo",
      "passes": false
    },
    {
      "id": "req-primes-json",
      "category": "functional",
      "priority": "critical",
      "description": "Output to primes.json",
      "status": "todo",
      "passes": false
    }
  ]
}
EOF
` + "```" + `
`, nil
	}

	// 2. TPM/Planner Role: Jira Plan (MUST be before Coding Agent heuristic)
	// The TPM prompt asks for JSON output and identifies itself as Technical Program Manager.
	if strings.Contains(lowerPrompt, "technical program manager") && strings.Contains(lowerPrompt, "json") {
		return `
` + "```json" + `
[
  {
    "title": "ID:[PRIMES] Prime Number Script",
    "description": "Implement primes.py to calculate primes < 10000 and output to primes.json.\nRepo: https://github.com/process-failed-successfully/recac-jira-e2e",
    "type": "Task",
    "acceptance_criteria": [
      "primes.py created",
      "primes.json generated with correct primes"
    ],
    "blocked_by": [],
    "children": []
  }
]
` + "```" + `
`, nil
	}

	// 3. Coding Agent: Implement primes.py
	if strings.Contains(lowerPrompt, "primes.py") || strings.Contains(lowerPrompt, "prime number script") {
		return `I will implement the prime number script.

` + "```bash" + `
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n < 2: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

primes = [i for i in range(10000) if is_prime(i)]
with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
EOF

# Run it to generate the JSON
python3 primes.py

# Mark as done
agent-bridge feature set req-primes-py --status done --passes true
agent-bridge feature set req-primes-json --status done --passes true
agent-bridge signal QA_PASSED true || touch QA_PASSED
` + "```" + `
`, nil
	}

	// 4. Manager Agent: Approve
	if strings.Contains(lowerPrompt, "manager agent") || strings.Contains(lowerPrompt, "review") {
		return `APPROVED
` + "```bash" + `
agent-bridge signal --privileged PROJECT_SIGNED_OFF true || touch PROJECT_SIGNED_OFF
` + "```" + `
`, nil
	}

	// Default fallback response
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
