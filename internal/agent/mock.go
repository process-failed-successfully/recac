package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// MockAgent is a simple mock agent for testing and mock mode
// It returns predefined responses without making actual API calls
type MockAgent struct {
	responsePrefix string
	forcedResponse string
	callCount      int
	mu             sync.Mutex
}

// NewMockAgent creates a new mock agent
func NewMockAgent() *MockAgent {
	return &MockAgent{
		responsePrefix: "Mock agent response",
	}
}

// SetResponse forces a specific response from the agent
func (m *MockAgent) SetResponse(response string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.forcedResponse = response
}

// Send implements the Agent interface
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	lowerPrompt := strings.ToLower(prompt)

	// Heuristic 1: TPM / Jira Ticket Generation
	// The prompt usually contains "Technical Program Manager" or "Ticket Generation"
	if strings.Contains(lowerPrompt, "technical program manager") || strings.Contains(lowerPrompt, "create a single ticket") {
		// Return a valid JSON list of tickets as expected by the Jira generator
		return `[
  {
    "id": "PRIMES",
    "summary": "Implement Prime Number Script",
    "description": "Implement primes.py to calculate primes < 10000 and output to primes.json. Verify count is 1229.",
    "type": "Task"
  }
]`, nil
	}

	// Heuristic 2: Primes Coding Task (Stateful)
	// Triggers when the agent is asked to implement the primes script
	if strings.Contains(lowerPrompt, "prime") || strings.Contains(lowerPrompt, "primes.py") {
		m.callCount++

		// Iteration 1: Write and Run Script
		if m.callCount == 1 {
			return `I will create the python script to calculate primes and run it.

` + "```bash" + `
cat << 'EOF' > primes.py
import json

primes = []
for num in range(2, 10000):
    is_prime = True
    for i in range(2, int(num ** 0.5) + 1):
        if num % i == 0:
            is_prime = False
            break
    if is_prime:
        primes.append(num)

with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
EOF

python3 primes.py
` + "```" + `
`, nil
		}

		// Iteration 2: Commit Results
		if m.callCount == 2 {
			return `I have generated the primes. Now I will commit the files.

` + "```bash" + `
git add primes.py primes.json
git commit -m "Implement primes calculation"
` + "```" + `
`, nil
		}

		// Iteration 3+: Done
		return `The task is complete. The primes.json file has been generated and committed.
agent-bridge feature set "Implement Prime Number Script" true
`, nil
	}

	// Heuristic 3: QA Agent
	if strings.Contains(lowerPrompt, "you are the qa agent") || strings.Contains(lowerPrompt, "qa_passed") {
		return "QA_PASSED", nil
	}

	// Heuristic 4: Manager Review / Sign-off
	// If the manager sees the QA report or is asked to review
	if strings.Contains(lowerPrompt, "qa report") || strings.Contains(lowerPrompt, "## your role - project manager") {
		return `The project looks good and QA passed.
agent-bridge signal PROJECT_SIGNED_OFF true --privileged
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
