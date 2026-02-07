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

	// 1. Initializer Agent - Sets up the repo
	// Check for "INITIALIZER" (uppercase) as used in prompts/templates/initializer.md
	// MOVED TO TOP to prevent TPM/Generic heuristics from catching "Application Specification" in the prompt
	// CRITICAL: We must be specific to the ROLE header to avoid matching "git init" in the history of other agents.
	if strings.Contains(prompt, "ROLE - INITIALIZER AGENT") {
		// Detect Primes scenario
		if strings.Contains(strings.ToLower(prompt), "prime") {
			return `
I will initialize the repository and create the feature list for the prime number script.

` + "```bash" + `
git init
git config user.email "you@example.com"
git config user.name "Your Name"

# Create feature list via agent-bridge import
cat << 'EOF' | agent-bridge import
{
  "project_name": "Prime Number Generator",
  "features": [
    {
      "id": "PRIMES",
      "category": "functional",
      "priority": "MVP",
      "description": "Implement a python script 'primes.py' that calculates primes < 10000 and outputs to 'primes.json'.",
      "status": "pending",
      "passes": false,
      "steps": [
        "Create primes.py",
        "Run python3 primes.py",
        "Verify primes.json exists"
      ],
      "dependencies": {
        "exclusive_write_paths": ["primes.py", "primes.json"],
        "read_only_paths": []
      }
    }
  ]
}
EOF
` + "```" + `
`, nil
		}

		return `
I will initialize the repository and import the plan.

` + "```bash" + `
git init
git config user.email "you@example.com"
git config user.name "Your Name"
agent-bridge import --file /app/ticket_plan.json
` + "```" + `
`, nil
	}

	// 2. TPM Agent - Generates the plan
	// Removed "Application Specification" check as it is too broad and appears in Initializer prompt
	if strings.Contains(prompt, "Technical Program Manager") {
		return `
[
  {
    "id": "PRIMES",
    "type": "Task",
    "title": "Implement prime number script",
    "description": "Implement a python script 'primes.py' that calculates primes < 10000 and outputs to 'primes.json'."
  }
]
`, nil
	}

	// 3. Coding Agent - Implements the feature
	// We detect the [PRIMES] ID or the file request
	if strings.Contains(prompt, "[PRIMES]") || strings.Contains(prompt, "primes.py") {
		return `
I will implement the prime number script as requested.

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

python3 primes.py
git add primes.py primes.json
git commit -m "Add primes.py and primes.json"
git push
agent-bridge feature set PRIMES --status implemented
` + "```" + `
`, nil
	}

	// 4. Default / Fallback
	// Return a mock response that shows the agent received the prompt
	fmt.Printf("[MockAgent] Hit Fallback! Prompt length: %d\nFull Prompt:\n%s\n", len(prompt), prompt)
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
