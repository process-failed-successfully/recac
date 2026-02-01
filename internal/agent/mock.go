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

	// Smart Mock Logic for Smoke Tests
	// 1. Ticket Generation Request (Prime Python Scenario)
	if strings.Contains(prompt, "ID:[PRIMES]") && strings.Contains(prompt, "JSON format") {
		return `[
  {
    "title": "[GEN] Create Prime Number Script",
    "description": "Create a python script named 'primes.py' that calculates primes < 10000 and outputs to 'primes.json'. ID:[PRIMES]",
    "type": "Task",
    "children": []
  }
]`, nil
	}

	// 2. Implementation Request (Writing the file)
	// Matches prompt asking to implement "PRIMES" or "primes.py"
	// Also check for [GEN] tag which appears in E2E tests
	if strings.Contains(prompt, "PRIMES") || strings.Contains(prompt, "primes.py") || strings.Contains(prompt, "[GEN]") {
		return `I will create the primes.py script and the json output as requested.

` + "```bash" + `
# Create the python script
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

primes_list = get_primes(10000)
with open("primes.json", "w") as f:
    json.dump({"primes": primes_list}, f)
EOF

# Run the script to generate the json
python3 primes.py

# Add and commit
git add primes.py primes.json
git commit -m "Add primes script and output"
` + "```" + `
`, nil
	}

	// Default Mock Response
	// We include a no-op bash block to ensure the executor doesn't trip the "no commands" circuit breaker
	// We strip backticks from the preview to avoid confusing the regex parser
	preview := strings.ReplaceAll(truncateString(prompt, 100), "`", "'")
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...\n\n```bash\n# no-op to prevent circuit breaker\necho 'mock agent alive'\n```",
		m.responsePrefix, len(prompt), preview)
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
