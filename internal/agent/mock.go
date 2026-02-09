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

	// 1. Initializer Heuristic (Project Setup)
	// If the prompt asks for a feature list, generate one.
	if strings.Contains(strings.ToLower(prompt), "feature list") && strings.Contains(prompt, "JSON") {
		return `I have analyzed the spec. Here is the feature list.

` + "```bash" + `
cat << 'EOF' > feature_list.json
{
  "project_name": "Prime Number Generator",
  "features": [
    {
      "id": "req-script-prints-primes",
      "description": "Implement a Python script to calculate prime numbers < 10000",
      "priority": "critical",
      "category": "core",
      "status": "pending"
    }
  ]
}
EOF

agent-bridge import feature_list.json
` + "```", nil
	}

	// 2. Loop Breaker Heuristic
	// If prompt suggests we are in a "nothing to commit" loop or "QA failed", we try to signal success if we've done work.
	if strings.Contains(prompt, "nothing to commit") || strings.Contains(prompt, "working tree clean") {
		return `It seems I have completed the task and there are no new changes. I will signal QA success.

` + "```bash" + `
agent-bridge signal --privileged QA_PASSED true
` + "```", nil
	}

	// 3. TPM Agent Heuristic (Ticket Generation)
	// Detects the planning phase where tickets need to be generated.
	if (strings.Contains(strings.ToLower(prompt), "create") && strings.Contains(strings.ToLower(prompt), "ticket")) && (strings.Contains(prompt, "primes.py") || strings.Contains(prompt, "[PRIMES]")) {
		return `I have analyzed the request. Here is the plan.

` + "```json" + `
[
  {
    "title": "ID:[PRIMES] Implement Prime Number Script",
    "description": "Implement a Python script to calculate prime numbers < 10000",
    "type": "Task"
  }
]
` + "```", nil
	}

	// 4. Coding Agent Heuristic (Prime Python Scenario)
	// Detects the specific "primes.py" request from the e2e/scenarios/prime_python.go
	if strings.Contains(prompt, "primes.py") || strings.Contains(prompt, "[PRIMES]") {
		// Otherwise, generate the code.
		return `I will implement the prime number generator script as requested.

` + "```bash" + `
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = [i for i in range(2, 10000) if is_prime(i)]
with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
EOF

# Run the script to generate the output
python3 primes.py

# Commit the changes
git add primes.py primes.json
git commit -m "Implement primes.py and generate primes.json"
` + "```", nil
	}

	// 5. QA Agent Heuristic
	// If asked to test or verify, we just say it passed.
	if strings.Contains(strings.ToLower(prompt), "run qa") || strings.Contains(strings.ToLower(prompt), "verify") {
		return `I have verified the implementation. All tests passed.

` + "```bash" + `
agent-bridge signal --privileged QA_PASSED true
` + "```", nil
	}

	// 6. Project Manager / Sign Off
	if strings.Contains(prompt, "sign off") || strings.Contains(prompt, "Project Signed Off") {
		return `The project is complete.

` + "```bash" + `
agent-bridge signal --privileged PROJECT_SIGNED_OFF true
` + "```", nil
	}

	// Default Fallback
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
