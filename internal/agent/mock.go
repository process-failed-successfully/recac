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

	// 1. Technical Program Manager (TPM) heuristic
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "TPM") {
		tickets := []map[string]interface{}{
			{
				"title":       "Implement prime number generator",
				"description": "Create a python script that prints the first n prime numbers.",
				"type":        "Task",
				"priority":    "High",
			},
		}
		responseBytes, _ := json.Marshal(tickets)
		return string(responseBytes), nil
	}

	// 2. Primes heuristic (Coding Agent)
	if strings.Contains(prompt, "ID:[PRIMES]") || strings.Contains(prompt, "primes.py") {
		script := `cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = [i for i in range(2, 50) if is_prime(i)]
with open('primes.json', 'w') as f:
    json.dump(primes, f)
EOF

python3 primes.py
git add primes.json
git commit -m "Add primes.json"
agent-bridge feature set req-script-runs-without-errors
agent-bridge signal --privileged PROJECT_SIGNED_OFF
`
		return fmt.Sprintf("```bash\n%s\n```", strings.TrimSpace(script)), nil
	}

	// 3. QA Agent heuristic
	if strings.Contains(prompt, "QA Agent") {
		return "All tests passed", nil
	}

	// 4. Reviewer heuristic
	if strings.Contains(prompt, "Reviewer") {
		return "LGTM", nil
	}

	// Default: Return a mock response that shows the agent received the prompt
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
