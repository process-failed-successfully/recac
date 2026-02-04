package agent

import (
	"context"
	"fmt"
	"strings"
)

// MockAgent is a smart mock agent for testing and mock mode.
// It implements heuristics to pass standard e2e scenarios like 'prime-python'.
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

// Send implements the Agent interface with scenario-specific logic
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// --- 0. Initializer (Feature Loading) ---
	if strings.Contains(prompt, "feature_list.json") || strings.Contains(prompt, "Initializer") {
		return `
I will initialize the project features.

` + "```bash" + `
#!/bin/bash
set -e

cat << 'EOF' > /tmp/features.json
{
    "features": [
        {
            "id": "PRIMES",
            "name": "Primes Script",
            "type": "requirement",
            "description": "Create a python script named 'primes.py' that calculates all prime numbers less than 10,000.",
            "status": "pending"
        }
    ]
}
EOF

agent-bridge import < /tmp/features.json
` + "```" + `
`, nil
	}

	// --- 1. Initializer / Ticket Generation ---
	if strings.Contains(prompt, "CRITICAL INSTRUCTION: You MUST create exactly ONE ticket") && strings.Contains(prompt, "ID:[PRIMES]") {
		// Return the JSON ticket list expected by the orchestrator
		return `
Here is the plan for the Prime Number Script:

` + "```json" + `
[
  {
    "id": "PRIMES",
    "title": "ID:[PRIMES] Create Prime Number Script",
    "description": "Create a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to 'primes.json'. Verify the output contains exactly 1229 primes.",
    "type": "Task",
    "status": "todo",
    "points": 1,
    "assigned_to": "agent"
  }
]
` + "```" + `
`, nil
	}

	// --- 2. Implementation Phase (Agent receiving the task) ---
	// The agent receives the ticket description which contains "Create a python script named 'primes.py'"
	if strings.Contains(prompt, "Create a python script named 'primes.py'") || strings.Contains(prompt, "calculate all prime numbers less than 10,000") {
		// Return the bash commands to implement the solution and signal completion
		return `
I will implement the prime number script as requested.

` + "```bash" + `
#!/bin/bash
set -e

# Create primes.py
cat << 'EOF' > primes.py
import json

def get_primes(n):
    primes = []
    for num in range(2, n):
        is_prime = True
        for i in range(2, int(num ** 0.5) + 1):
            if num % i == 0:
                is_prime = False
                break
        if is_prime:
            primes.append(num)
    return primes

primes = get_primes(10000)
print(f"Found {len(primes)} primes")

with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
EOF

# Run it to generate the json
python3 primes.py

# Git operations
git add primes.py primes.json
git commit -m "feat: implement primes.py and generate primes.json" || echo "No changes to commit"
git push origin HEAD || echo "Push skipped"

# Signal completion
agent-bridge feature update --id PRIMES --status in_progress
agent-bridge signal --signal QA_PASSED
` + "```" + `
`, nil
	}

	// --- 3. QA / Review Phase ---
	if strings.Contains(prompt, "YOUR ROLE: QA Agent") || strings.Contains(prompt, "verify the changes") {
		return `
The changes look correct. The 'primes.py' script is implemented and 'primes.json' is generated.

` + "```bash" + `
agent-bridge signal --signal QA_PASSED
` + "```" + `
`, nil
	}

	// --- 4. Manager / Sign-off Phase ---
	if strings.Contains(prompt, "YOUR ROLE: Manager Agent") || strings.Contains(prompt, "review the project") {
		return `
The project requirements are met.

` + "```bash" + `
agent-bridge signal --signal PROJECT_SIGNED_OFF
` + "```" + `
`, nil
	}

	// --- Default / Fallback ---
	// Just echo to avoid hanging, but try to be helpful if it looks like a generic command request
	return fmt.Sprintf("%s:\n\nI received your prompt (%d characters). I am running in MOCK MODE. If you expected a specific behavior, please update the MockAgent logic.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100)), nil
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
