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

	lowerPrompt := strings.ToLower(prompt)

	// Heuristic 1: TPM/Architect Phase (Planning)
	// Triggers when asking for JSON tickets
	if (strings.Contains(lowerPrompt, "architect") || strings.Contains(lowerPrompt, "tpm")) && strings.Contains(lowerPrompt, "json") {
		tickets := []map[string]interface{}{
			{
				"summary":     "ID:[PRIMES] Implement Python Script",
				"description": "Implement a python script that calculates prime numbers.",
				"type":        "Story",
				"children":    []interface{}{}, // Empty array for children
			},
		}
		jsonBytes, _ := json.Marshal(tickets)
		// Return strictly formatted JSON as expected by the CLI
		return fmt.Sprintf("```json\n%s\n```", string(jsonBytes)), nil
	}

	// Heuristic 2: Coding Phase (Implementation)
	// Triggers for the specific prime number task
	if strings.Contains(lowerPrompt, "prime") && strings.Contains(lowerPrompt, "python") {
		script := `
git config user.email "agent@recac.io"
git config user.name "RECAC Agent"

cat <<EOF > primes.py
def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

print([x for x in range(100) if is_prime(x)])
EOF

python3 primes.py
git add primes.py
git commit -m "Implement primes.py"
`
		// Construct the JSON response the agent expects
		responseObj := map[string]string{
			"response": fmt.Sprintf("I will implement the python script.\n\n```bash\n%s\n```", script),
		}
		jsonBytes, _ := json.Marshal(responseObj)
		return string(jsonBytes), nil
	}

	// Fallback: Return a mock response that shows the agent received the prompt
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
