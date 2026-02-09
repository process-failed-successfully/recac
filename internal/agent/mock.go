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
// It uses heuristics to return appropriate responses for the smoke test lifecycle
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// --- Heuristics for Smoke Test Scenarios ---

	// 1. Technical Program Manager (TPM) - Ticket Generation
	// Trigger: "Technical Program Manager" or "TPM" in prompt
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "TPM") {
		// Return JSON array of tickets as expected by recac jira generate-from-spec
		return `[
  {
    "id": "ID:[PRIMES]",
    "title": "Implement Prime Number Generator",
    "description": "Create a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to 'primes.json'.",
    "type": "Task",
    "status": "Todo",
    "acceptance_criteria": [
      "req-script-exists: Script primes.py exists",
      "req-json-exists: Output file primes.json exists",
      "req-correct-count: primes.json contains exactly 1229 primes"
    ]
  }
]`, nil
	}

	// 2. Initializer - Feature List Import
	// Trigger: "CREATE FEATURE_LIST.JSON" or "Initializer" role
	if strings.Contains(prompt, "CREATE FEATURE_LIST.JSON") || strings.Contains(prompt, "Initializer") {
		return `
cat << 'EOF' > feature_list.json
{
  "project_name": "prime-python",
  "features": [
    {
      "id": "req-script-exists",
      "description": "Script primes.py exists",
      "status": "pending"
    },
    {
      "id": "req-json-exists",
      "description": "Output file primes.json exists",
      "status": "pending"
    },
    {
      "id": "req-correct-count",
      "description": "primes.json contains exactly 1229 primes",
      "status": "pending"
    }
  ]
}
EOF
agent-bridge import < feature_list.json
`, nil
	}

	// 3. QA Agent - Verification and Sign-off
	// Trigger: "QA AGENT" role
	if strings.Contains(prompt, "QA AGENT") {
		return `
# Run validation (assuming coding agent created it)
if [ -f primes.py ]; then
    python3 primes.py
    # QA_PASSED might not be privileged, but using --privileged helps if logic changes
    # We use explicit key/value syntax
    agent-bridge signal QA_PASSED true || agent-bridge signal --privileged QA_PASSED true
else
    echo "primes.py not found"
    exit 1
fi
`, nil
	}

	// 4. Project Manager - Final Sign-off
	// Trigger: "PROJECT MANAGER" role
	if strings.Contains(prompt, "PROJECT MANAGER") {
		// PM needs to sign off to exit the loop
		return `agent-bridge signal --privileged PROJECT_SIGNED_OFF true`, nil
	}

	// 5. Coding Agent - Implementation
	// Trigger: "[PRIMES]" (Ticket ID) or "CODING AGENT" or "primes.py" context
	if strings.Contains(prompt, "[PRIMES]") || strings.Contains(prompt, "CODING AGENT") || strings.Contains(prompt, "primes.py") {
		return `
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
with open('primes.json', 'w') as f:
    json.dump({'primes': primes}, f)
EOF

python3 primes.py

# Mark features as done
agent-bridge feature set req-script-exists --status done --passes true || echo "Feature set failed"
agent-bridge feature set req-json-exists --status done --passes true || echo "Feature set failed"
agent-bridge feature set req-correct-count --status done --passes true || echo "Feature set failed"

git add primes.py primes.json
git diff --cached --quiet || git commit -m "Implement primes.py"
`, nil
	}

	// Default / Fallback Response
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
