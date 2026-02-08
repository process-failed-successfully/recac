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
// Signature matches expected factory usage if needed, or we adapt interface
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

	// Heuristics for E2E scenarios

	// 1. Initializer Agent (Feature List)
	if strings.Contains(strings.ToUpper(prompt), "ROLE - INITIALIZER AGENT") || strings.Contains(prompt, "feature list") {
		if strings.Contains(prompt, "primes.py") || strings.Contains(prompt, "[PRIMES]") {
			return `{
  "features": [
    {
      "id": "req-primes",
      "name": "Implement Prime Number Script",
      "description": "Calculate primes < 10000",
      "priority": "1",
      "dependencies": {"depends_on_ids": []}
    }
  ]
}`, nil
		}
	}

	// 2. Technical Program Manager (TPM) - Check BEFORE Coding Agent
	if strings.Contains(strings.ToUpper(prompt), "TECHNICAL PROGRAM MANAGER") {
		if strings.Contains(prompt, "[PRIMES]") || strings.Contains(prompt, "primes.py") {
			return `[
  {
    "id": "PRIMES",
    "summary": "[PRIMES] Create Prime Number Script",
    "description": "Implement a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to a file named 'primes.json'.",
    "type": "Task",
    "dependencies": []
  }
]`, nil
		}
		// Return empty or valid JSON ticket list to avoid blocking generic calls
		return `[]`, nil
	}

	// 3. Coding Agent / Developer (Primes)
	// We check for keywords related to the Primes scenario
	if containsAny(prompt, []string{"[PRIMES]", "primes.py", "Implement Primes", "Prime Number Script"}) {
		// Return the implementation
		return `Here is the python script to calculate primes.

` + "```bash" + `
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = [i for i in range(10000) if is_prime(i)]
with open("primes.json", "w") as f:
    json.dump({"primes": primes}, f)
EOF

# Run it to generate the json
python3 primes.py

# Add to git
git add primes.py primes.json
git commit -m "Add primes script and output"
agent-bridge feature set --id req-primes --status done
` + "```" + `
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

func containsAny(s string, substrs []string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
