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

	// Heuristic 1: Prime-Python
	// 'id:[primes]', 'generate primes', 'primes.json', OR ('prime' AND 'python')
	// Exclude 'technical program manager'
	if !strings.Contains(lowerPrompt, "technical program manager") {
		if strings.Contains(lowerPrompt, "id:[primes]") ||
			strings.Contains(lowerPrompt, "generate primes") ||
			strings.Contains(lowerPrompt, "primes.json") ||
			(strings.Contains(lowerPrompt, "prime") && strings.Contains(lowerPrompt, "python")) {

			// Return JSON response containing bash script to create primes.py
			script := `
cat <<EOF > primes.py
def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

if __name__ == "__main__":
    import sys
    n = int(sys.argv[1]) if len(sys.argv) > 1 else 10
    print([x for x in range(n) if is_prime(x)])
EOF
`
			responseMap := map[string]string{
				"command": script,
			}
			bytes, _ := json.Marshal(responseMap)
			return string(bytes), nil
		}
	}

	// Heuristic 2: Git Lead -> git checkout -b
	if strings.Contains(lowerPrompt, "git lead") {
		return "git checkout -b feature/primes", nil
	}

	// Heuristic 3: TPM
	// 'json' AND ('technical program manager' OR 'architect' OR 'tpm')
	if strings.Contains(lowerPrompt, "json") &&
		(strings.Contains(lowerPrompt, "technical program manager") ||
			strings.Contains(lowerPrompt, "architect") ||
			strings.Contains(lowerPrompt, "tpm")) {

		// Return JSON array of ticketNode objects
		tickets := []map[string]string{
			{
				"id":          "PRIMES",
				"type":        "Story",
				"title":       "Generate Primes",
				"description": "Write a python script to generate primes.",
			},
		}
		bytes, _ := json.Marshal(tickets)
		return string(bytes), nil
	}

	// Heuristic 4: Commit Message
	if strings.Contains(lowerPrompt, "commit message") {
		return "feat: Implement primes.py", nil
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
