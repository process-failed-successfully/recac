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
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	lowerPrompt := strings.ToLower(prompt)

	// 1. Initializer Agent or Feature List Request
	// Note: We check for "create feature_list.json" or explicit role to avoid false positives
	// when the prompt merely *contains* the file content context.
	if strings.Contains(lowerPrompt, "initializer agent") || strings.Contains(lowerPrompt, "create feature_list.json") {
		return `I will create the feature list.

` + "```bash" + `
cat << 'EOF' > feature_list.json
{
  "project_name": "ID:[PRIMES] Prime Number Script",
  "features": [
    {
      "id": "req-primes",
      "category": "functional",
      "priority": "critical",
      "description": "Implement primes.py to calculate primes < 10000",
      "status": "pending",
      "passes": false,
      "steps": [
        "Create primes.py",
        "Run primes.py to generate primes.json",
        "Verify output"
      ]
    }
  ]
}
EOF
` + "```" + `

Files created.
`, nil
	}

	// 2. Manager/QA Approval
	if strings.Contains(lowerPrompt, "qa agent") {
		return `I have verified the implementation.

` + "```bash" + `
agent-bridge signal QA_PASSED true
` + "```" + `

All checks passed.
`, nil
	}

	if strings.Contains(lowerPrompt, "manager agent") || strings.Contains(lowerPrompt, "review qa report") {
		return `APPROVED. I approve this project.

` + "```bash" + `
agent-bridge signal --privileged PROJECT_SIGNED_OFF true
` + "```" + `

Great work.
`, nil
	}

	// 3. [PRIMES] Task Handling
	if strings.Contains(strings.ToUpper(prompt), "[PRIMES]") {
		// A. Ticket Generation Request
		if strings.Contains(lowerPrompt, "generate") || strings.Contains(lowerPrompt, "ticket") || strings.Contains(lowerPrompt, "json") {
			return `[
  {
    "title": "ID:[PRIMES] Prime Number Script",
    "description": "Implement a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to 'primes.json'.",
    "type": "Task"
  }
]`, nil
		}

		// B. Implementation Request (Default for PRIMES)
		return `I will implement the prime number script.

` + "```bash" + `
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

primes = [p for p in range(10000) if is_prime(p)]
output = {"primes": primes}

with open("primes.json", "w") as f:
    json.dump(output, f)
EOF

# Run the script to generate the output
python3 primes.py

# Mark feature as done
agent-bridge feature set req-primes --status done --passes true

# Commit the files
git add primes.py primes.json
git commit -m "Implement primes.py and generate primes.json"
git push
` + "```" + `

I have implemented the script, generated the JSON, and committed the changes.
`, nil
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
