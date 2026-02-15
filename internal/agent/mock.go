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

	lowerPrompt := strings.ToLower(prompt)

	// --- Heuristics ---

	// 1. Initializer Agent
	if strings.Contains(lowerPrompt, "initializer agent") || strings.Contains(lowerPrompt, "plan") {
		return "```json\n{\"plan\": [\"Implement prime number generator\", \"Write tests\", \"Verify output\"]}\n```\nHere is the plan to implement the requested feature.", nil
	}

	// 2. Coding Agent (Primes Scenario)
	// We check for "prime" to catch the specific scenario requirement
	if strings.Contains(lowerPrompt, "coding agent") || strings.Contains(lowerPrompt, "write code") || strings.Contains(lowerPrompt, "prime") {
		return `I will create the primes.py script and run it to generate primes.json.

` + "```bash" + `
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

primes = [n for n in range(2, 10001) if is_prime(n)]

with open('primes.json', 'w') as f:
    json.dump({'primes': primes}, f)

print(f'Found {len(primes)} primes')
EOF

# Run the script to generate the json
python3 primes.py
` + "```" + `

I have implemented the script and generated the json file.
`, nil
	}

	// 3. QA Agent
	if strings.Contains(lowerPrompt, "qa agent") || strings.Contains(lowerPrompt, "verify") {
		return "QA_PASSED\nAll tests passed successfully.", nil
	}

	// 4. Manager Agent
	if strings.Contains(lowerPrompt, "manager agent") || strings.Contains(lowerPrompt, "review") {
		return "PROJECT_SIGNED_OFF\nThe project meets all requirements.", nil
	}

	// Default fallback
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
