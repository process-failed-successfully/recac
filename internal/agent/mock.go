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

	// 1. Ticket Generation Prompt (from PrimePythonScenario.AppSpec via recac jira generate-from-spec)
	// Requires []ticketNode (JSON Array)
	if strings.Contains(prompt, "ID:[PRIMES]") && strings.Contains(prompt, "MUST create exactly ONE ticket") {
		return `[
  {
    "title": "ID:[PRIMES] Create Prime Number Script",
    "description": "Create a python script named 'primes.py'. It must calculate all prime numbers less than 10,000 and output to a file named 'primes.json'.\n\nREQUIRED FEATURES:\n- Implement prime calculation logic in primes.py\n- Output results to primes.json\n- Validate that the output file contains a 'primes' list\n- Verify that exactly 1229 primes are calculated\n- Commit primes.json to the repository",
    "type": "Task",
    "acceptance_criteria": [
      "primes.py created",
      "primes.json created with 1229 primes",
      "files committed"
    ],
    "children": []
  }
]`, nil
	}

	// 2. Feature List Planning Prompt (from recac init / planner.md)
	// Requires FeatureList (JSON Object)
	if strings.Contains(prompt, "Create a JSON object containing a feature list") {
		return `I have created the feature list.

` + "```bash" + `
cat << 'EOF' | agent-bridge import
{
  "project_name": "Prime Number Generator",
  "features": [
    {
      "id": "PRIMES",
      "category": "functional",
      "description": "Calculate prime numbers using Python. Create a file named primes.py that prints prime numbers up to 10,000.",
      "status": "pending",
      "steps": [
        "Create primes.py",
        "Implement prime calculation logic",
        "Print primes to stdout"
      ],
      "dependencies": {
        "depends_on_ids": [],
        "exclusive_write_paths": [],
        "read_only_paths": []
      }
    }
  ]
}
EOF
` + "```" + `
`, nil
	}

	// 3. Implementation Prompt (from CodingAgent)
	// Requires Bash Script
	if isImplementationPrompt(prompt) {
		return `Here is the solution:

` + "```bash" + `
# Create primes.py
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = [n for n in range(10001) if is_prime(n)]
print(f"Calculated {len(primes)} primes.")

with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
EOF

# Verify and Commit
python3 primes.py
git add primes.py primes.json
git commit -m "Add primes script and output"
` + "```" + `

Done.
`, nil
	}

	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100))
	return response, nil
}

func isImplementationPrompt(prompt string) bool {
	return len(prompt) > 0 && (contains(prompt, "primes.py") || contains(prompt, "Calculate prime numbers"))
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
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
