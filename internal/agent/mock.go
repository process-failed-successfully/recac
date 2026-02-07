package agent

import (
	"context"
	"fmt"
	"strings"
)

// MockAgent is a simple mock agent for testing and mock mode
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

	// Heuristics for Prime Python Scenario
	if strings.Contains(prompt, "Technical Program Manager") && (strings.Contains(prompt, "[PRIMES]") || strings.Contains(prompt, "primes.py")) {
		return m.generatePrimesTPMResponse(), nil
	}

	if strings.Contains(prompt, "[PRIMES]") || strings.Contains(prompt, "primes.py") {
		return m.generatePrimesScript(), nil
	}

	// Heuristics for QA/Manager
	if strings.Contains(prompt, "QA") || strings.Contains(prompt, "Review") || strings.Contains(prompt, "verify") {
		return "QA Checks Passed. No issues found.", nil
	}

	// Return a mock response that shows the agent received the prompt
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100))
	return response, nil
}

func (m *MockAgent) generatePrimesScript() string {
	return `
I will create the primes script as requested.

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
    json.dump({"primes": primes}, f)
EOF

# Run the script to generate output
python3 primes.py

# Verify the output count (should be 1229)
python3 -c "import json; print(f'Primes count: {len(json.load(open('primes.json'))['primes'])}')"

# Add files to git
git add primes.py primes.json
git commit -m "Implement primes script" || echo "No changes to commit"

# Signal completion to stop the runner
agent-bridge signal COMPLETED true
` + "```" + `
`
}

func (m *MockAgent) generatePrimesTPMResponse() string {
	return "```json\n" + `
[
  {
    "title": "ID:[PRIMES] Implement Prime Number Script",
    "description": "Develop a Python script to generate prime numbers and save them to a JSON file.\nRepo: https://github.com/process-failed-successfully/recac-jira-e2e",
    "type": "Epic",
    "children": [
      {
        "title": "Implement Primes Logic",
        "description": "Write the is_prime function and main logic to generate primes up to 10000.\nRepo: https://github.com/process-failed-successfully/recac-jira-e2e",
        "type": "Story",
        "acceptance_criteria": [
          "is_prime function correctly identifies primes",
          "Script generates primes up to 10000"
        ]
      },
      {
        "title": "Save Primes to JSON",
        "description": "Save the generated primes list to primes.json.\nRepo: https://github.com/process-failed-successfully/recac-jira-e2e",
        "type": "Story",
        "acceptance_criteria": [
          "Output file is valid JSON",
          "File contains exactly 1229 primes"
        ],
        "blocked_by": ["Implement Primes Logic"]
      }
    ]
  }
]
` + "\n```"
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
