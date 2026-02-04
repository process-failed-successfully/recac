package agent

import (
	"context"
	"fmt"
	"strings"
)

// MockAgent is a smart mock agent for testing and mock mode
// It returns predefined responses based on heuristics to simulate a real agent
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
	// Debug logging for CI diagnosis
	fmt.Printf("[MockAgent] Received prompt (%d chars): %s...\n", len(prompt), truncateString(prompt, 100))

	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	promptLower := strings.ToLower(prompt)

	// 1. Initializer Role (Setup feature list)
	// Check for "initializer agent" OR ("initialize" AND "feature_list.json")
	if strings.Contains(promptLower, "initializer agent") || (strings.Contains(promptLower, "initialize") && strings.Contains(promptLower, "feature_list.json")) {
		return `
'''bash
cat << 'EOF' > feature_list.json
{
  "project_name": "prime-python",
  "features": [
    {
      "id": "1",
      "description": "Calculate primes",
      "status": "todo",
      "passes": false
    }
  ]
}
EOF
# Import features to DB immediately
cat feature_list.json | agent-bridge import
'''
`, nil
	}

	// 2. Ticket/Plan Generation (PM Role - if used)
	if strings.Contains(promptLower, "technical program manager") || strings.Contains(promptLower, "generate tickets") {
		return `
[
  {
    "title": "ID:[PRIMES] Implement Prime Number Script",
    "type": "story",
    "description": "Implement primes.py to calculate primes under 10000"
  }
]
`, nil
	}

	// 3. QA Role
	if strings.Contains(promptLower, "qa agent") || strings.Contains(promptLower, "verify the project") {
		return `
'''bash
agent-bridge signal QA_PASSED true
'''
`, nil
	}

	// 4. Manager Role (Sign-off)
	if strings.Contains(promptLower, "project manager") || strings.Contains(promptLower, "qa report") {
		return `
'''bash
agent-bridge signal PROJECT_SIGNED_OFF true
'''
`, nil
	}

	// 5. Completion Check (Loop breaker)
	// If git says "nothing to commit", we assume we are done with the implementation loop.
	if strings.Contains(promptLower, "nothing to commit") {
		return `
'''bash
# Mark feature as done
agent-bridge feature set 1 --status done --passes true
# Signal completion to break loop
agent-bridge signal QA_PASSED true
agent-bridge signal COMPLETED true
'''
`, nil
	}

	// 6. Implementation (Prime Python Scenario)
	// Matches specific keywords for the prime number task
	if strings.Contains(promptLower, "primes.py") || strings.Contains(promptLower, "calculate primes") || strings.Contains(prompt, "[PRIMES]") {
		return `
'''bash
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n < 2:
        return False
    for i in range(2, int(n ** 0.5) + 1):
        if n % i == 0:
            return False
    return True

primes = [x for x in range(10000) if is_prime(x)]
with open("primes.json", "w") as f:
    json.dump({"primes": primes}, f)
EOF

# Run it to generate the json
python3 primes.py

# Verify output exists
ls -l primes.json

# Commit
git add .
git commit -m "Implement primes.py" || echo "Nothing to commit"
'''
`, nil
	}

	// Default response: Return a benign command to avoid "NO-OP LOOP" circuit breaker
	response := fmt.Sprintf("%s:\n\nI received your prompt. \n'''bash\necho 'Mock Agent: Processing...'\n'''", m.responsePrefix)
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
