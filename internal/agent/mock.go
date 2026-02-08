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

	// Debug logging to help identify why heuristics might fail in CI
	// Truncate if too long to avoid spamming logs excessively
	debugPrompt := prompt
	if len(debugPrompt) > 200 {
		debugPrompt = debugPrompt[:200] + "..."
	}
	fmt.Fprintf(os.Stderr, "[MockAgent] Received prompt (%d chars): %s\n", len(prompt), debugPrompt)

	// Heuristics for E2E Tests

	// 1. TPM Role - Ticket Generation for [PRIMES] (Used by 'recac jira generate-from-spec')
	// We check for [PRIMES] OR primes.py to be more robust against prompt variations
	if strings.Contains(prompt, "Technical Program Manager") && (strings.Contains(prompt, "[PRIMES]") || strings.Contains(prompt, "primes.py")) {
		return "```json\n" + `
[
  {
    "title": "ID:[PRIMES] Implement Prime Number Script",
    "description": "Implement a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to a file named 'primes.json'.",
    "type": "Task",
    "children": []
  }
]
` + "```", nil
	}

	// 2. Initializer Role - Feature Generation for [PRIMES] (Used by 'recac start' session)
	// Triggered by "INITIALIZER AGENT" or "Initializer Agent" prompt
	if (strings.Contains(prompt, "INITIALIZER AGENT") || strings.Contains(prompt, "Initializer Agent")) && (strings.Contains(prompt, "[PRIMES]") || strings.Contains(prompt, "primes.py")) {
		return `I will generate the feature list for the primes task.

` + "```bash" + `
cat << 'EOF' | agent-bridge import
{
  "project_name": "Primes",
  "features": [
    {
      "id": "PRIMES",
      "category": "functional",
      "priority": "MVP",
      "description": "Implement a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to a file named 'primes.json'.",
      "status": "pending",
      "steps": [
        "Create primes.py",
        "Run primes.py",
        "Verify primes.json exists",
        "Commit changes"
      ],
      "passes": false,
      "dependencies": {
        "depends_on_ids": [],
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

	// 2. Coding Agent Role - Implementation for [PRIMES]
	// Detect via task context or ID
	if (strings.Contains(prompt, "ID:[PRIMES]") || strings.Contains(prompt, "primes.py")) &&
	   (strings.Contains(prompt, "You are a software engineer") || strings.Contains(prompt, "CODING AGENT")) {
		return `I will implement the primes script.

` + "```bash" + `
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n <= 1: return False
    if n <= 3: return True
    if n % 2 == 0 or n % 3 == 0: return False
    i = 5
    while i * i <= n:
        if n % i == 0 or n % (i + 2) == 0: return False
        i += 6
    return True

primes = [i for i in range(10000) if is_prime(i)]

with open("primes.json", "w") as f:
    json.dump({"primes": primes}, f)
EOF

python3 primes.py
git add primes.py primes.json
git commit -m "Implement primes.py"
` + "```" + `
`, nil
	}

	// Return a mock response that shows the agent received the prompt
	// This allows the session to run without requiring real API keys
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
