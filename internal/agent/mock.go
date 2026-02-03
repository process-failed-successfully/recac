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

	// 1. Initializer / Ticket Generation Logic
	if strings.Contains(prompt, "CRITICAL INSTRUCTION FOR TICKET GENERATION") {
		// Return JSON ticket list
		return `[
  {
    "id": "[PRIMES]",
    "type": "Task",
    "summary": "Create Prime Number Script",
    "description": "Implement primes.py to calculate primes < 10000 and output to primes.json"
  }
]`, nil
	}

	// 2. Implementation Logic (Prime Python Scenario)
	// Heuristic: Check for prime keywords but exclude the Initializer phase which also mentions them
	if (strings.Contains(prompt, "primes.py") || strings.Contains(prompt, "prime numbers")) && !strings.Contains(prompt, "INITIALIZER") {
		// We need to return the implementation script
		// The script should:
		// 1. Create primes.py
		// 2. Run primes.py
		// 3. git add/commit
        // 4. Signal completion via agent-bridge

		return `
Here is the implementation for the prime number script:

'''bash
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

primes = [x for x in range(10000) if is_prime(x)]

with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
EOF

python3 primes.py
git add -f primes.json
git add primes.py
git commit -m "Add prime number script"
'''

I have implemented the script.
`, nil
	}

    // 3. Completion Check
    if strings.Contains(prompt, "nothing to commit") || strings.Contains(prompt, "Nothing to commit") {
         return "'''bash\nagent-bridge signal COMPLETED true\n'''", nil
    }

    // 4. Role Checks (QA/Manager)
    if strings.Contains(prompt, "YOUR ROLE - QA AGENT") {
         return "'''bash\nagent-bridge signal QA_PASSED true\n'''", nil
    }
    if strings.Contains(prompt, "YOUR ROLE - PROJECT MANAGER") {
         return "'''bash\nagent-bridge signal PROJECT_SIGNED_OFF true\n'''", nil
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
