package agent

import (
	"context"
	"encoding/json"
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

	// Heuristics for E2E Tests

	// 1. Initializer / Architect Phase (Feature List)
	// The prompt usually asks to break down requirements or list features.
	// CRITICAL: Check this BEFORE the Coding Phase, as the Initializer prompt also contains "PRIMES" (the ID).
	lowerPrompt := strings.ToLower(prompt)
	if strings.Contains(lowerPrompt, "feature list") ||
		strings.Contains(lowerPrompt, "break down") ||
		strings.Contains(lowerPrompt, "create exactly one ticket") ||
		strings.Contains(lowerPrompt, "create a single ticket") {
		// Return a list of tickets including PRIMES
		tickets := []map[string]string{
			{
				"id":          "PRIMES",
				"type":        "Task",
				"title":       "Create Prime Number Script",
				"description": "Calculate primes up to 10000",
			},
		}
		bytes, _ := json.Marshal(tickets)
		return string(bytes), nil
	}

	// 2. Coding Phase (PRIMES)
	// Only trigger if it's NOT an initializer prompt (which we handled above)
	if strings.Contains(prompt, "PRIMES") || strings.Contains(lowerPrompt, "prime number script") {
		// Return the bash script to create the python file and run it
		// The python script generates primes.json
		return `I will implement the prime number script.

` + "```bash" + `
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = [x for x in range(1, 10000) if is_prime(x)]
with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
EOF

# Run the script to generate the json
python3 primes.py

# Commit the results
git add primes.py primes.json
git commit -m "Implement prime number script"
` + "```" + `
`, nil
	}

	// 3. QA Phase
	if strings.Contains(lowerPrompt, "qa") || strings.Contains(lowerPrompt, "quality assurance") {
		return "QA Passed.\n`agent-bridge signal --id QA_PASSED`", nil
	}

	// 4. Manager Phase
	if strings.Contains(lowerPrompt, "manager") || strings.Contains(lowerPrompt, "sign off") {
		return "Project Signed Off.\n`agent-bridge signal --id PROJECT_SIGNED_OFF`", nil
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
