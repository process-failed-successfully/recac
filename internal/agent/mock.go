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

	// 1. Initializer / Feature List Phase
	// Check for "Initialize" or "feature_list.json" in the prompt
	if strings.Contains(lowerPrompt, "initialize") || strings.Contains(lowerPrompt, "feature_list.json") {
		return m.handleInitializer(), nil
	}

	// 2. Prime Python Scenario
	// Check for [PRIMES] tag or primes.py filename
	if strings.Contains(prompt, "[PRIMES]") || strings.Contains(lowerPrompt, "primes.py") {
		return m.handlePrimes(), nil
	}

	// 3. Manager/QA (Just pass)
	if strings.Contains(lowerPrompt, "qa agent") || strings.Contains(lowerPrompt, "project manager") {
		return "LGTM. QA Passed.", nil
	}

	// Return a mock response that shows the agent received the prompt
	// This allows the session to run without requiring real API keys
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100))
	return response, nil
}

func (m *MockAgent) handleInitializer() string {
	return `I will initialize the feature list.
` + "```bash" + `
cat << 'EOF' > feature_list.json
[
  {"id": "req-primes", "description": "Calculate primes", "status": "todo"}
]
EOF

# Ensure agent-bridge exists (optional check)
if command -v agent-bridge &> /dev/null; then
    agent-bridge update --file feature_list.json
fi
` + "```"
}

func (m *MockAgent) handlePrimes() string {
	// The python script must generate exactly 1229 primes < 10000
	pythonScript := `
import json

def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = [p for p in range(10000) if is_prime(p)]
with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
`
	return `I will implement the primes script.
` + "```bash" + `
# Configure Git
git config user.email "bot@recac.com"
git config user.name "Recac Bot"

# Create Script
cat << 'EOF' > primes.py
` + pythonScript + `
EOF

# Run Script
python3 primes.py

# Commit
git add primes.py primes.json
git commit -m "Add primes.py and primes.json" || echo "Nothing to commit"

# Update status (if agent-bridge exists)
if command -v agent-bridge &> /dev/null; then
    agent-bridge update "req-primes" --status done
fi
` + "```"
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
