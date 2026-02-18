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

	// Smart Mock Logic for E2E Tests
	lowerPrompt := strings.ToLower(prompt)

	// Debug logging for troubleshooting
	fmt.Printf("[MockAgent] Received prompt (len=%d): %s\n", len(prompt), truncateString(prompt, 200))

	// 1. Ticket Generation (Planning Phase)
	// Check for keywords indicating ticket creation request
	if (strings.Contains(lowerPrompt, "create a single ticket") || strings.Contains(lowerPrompt, "json format")) &&
		(strings.Contains(lowerPrompt, "primes.py") || strings.Contains(lowerPrompt, "prime number script")) {
		return m.generateTicketPlanResponse(), nil
	}

	// 2. Implementation Phase (Coding)
	// Check for implementation request
	// Also check for "repo: skip" combined with "prime" as a robust fallback for smoke tests
	if strings.Contains(lowerPrompt, "primes.py") ||
	   strings.Contains(lowerPrompt, "prime number script") ||
	   (strings.Contains(lowerPrompt, "prime") && strings.Contains(lowerPrompt, "skip")) {
		return m.generatePrimesScriptResponse(), nil
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

func (m *MockAgent) generateTicketPlanResponse() string {
	return `[
  {
    "title": "Implement Prime Number Script",
    "description": "Create a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to 'primes.json'.\n\nFeatures:\n- Calculate primes < 10,000\n- Output to primes.json in JSON format {\"primes\": [...]}\n- Commit both files",
    "type": "Task",
    "id": "PRIMES"
  }
]`
}

func (m *MockAgent) generatePrimesScriptResponse() string {
	script := `
import json

primes = []
for num in range(2, 10000):
    for i in range(2, int(num**0.5) + 1):
        if (num % i) == 0:
            break
    else:
        primes.append(num)

with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
`
	return fmt.Sprintf(`I will implement the prime number script as requested.

%s
cat << 'EOF' > primes.py
%s
EOF

# Execute the script to generate output
python3 primes.py

# Configure git if needed (CI fallback)
git config user.email "agent@recac.io" || true
git config user.name "RECAC Agent" || true

# Commit and Push
git add primes.py primes.json
git commit -m "Implement primes.py and add output" || echo "Nothing to commit"
git push || echo "Push failed (might be local mode)"

# Mark all pending features as done
agent-bridge feature list --status pending --format json | jq -r '.[].id' | xargs -I {} agent-bridge feature set {} --status done
%s
`, "```bash", script, "```")
}
