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
	callCount      int
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

	m.callCount++
	lowerPrompt := strings.ToLower(prompt)

	// Heuristic: Manager Review / QA
	if strings.Contains(lowerPrompt, "qa report") || strings.Contains(lowerPrompt, "project manager") || strings.Contains(lowerPrompt, "manager review") {
		return "Approved.\n```bash\nagent-bridge signal PROJECT_SIGNED_OFF true\n```", nil
	}

	// Heuristic: Prime Number Script (for prime-python scenario)
	if (strings.Contains(lowerPrompt, "primes") || strings.Contains(lowerPrompt, "generate primes")) && !strings.Contains(lowerPrompt, "def generate_primes") {
		return `I will implement the primes script.

` + "```bash" + `
cat << 'EOF' > primes.py
import json

def generate_primes(n):
    primes = []
    for i in range(2, n + 1):
        is_prime = True
        for j in range(2, int(i ** 0.5) + 1):
            if i % j == 0:
                is_prime = False
                break
        if is_prime:
            primes.append(i)
    return primes

if __name__ == "__main__":
    primes = generate_primes(9999)
    with open("primes.json", "w") as f:
        json.dump({"primes": primes}, f)
EOF

python3 primes.py
git add primes.py primes.json
git commit -m "feat: Implement primes.py"
` + "```" + `
`, nil
	}

	// Heuristic: Technical Program Manager (Ticket Generation)
	if strings.Contains(lowerPrompt, "technical program manager") {
		return `[{"id":"TASK-1", "summary":"Implement primes", "description":"Implement a python script to generate prime numbers.", "type":"Task"}]`, nil
	}

	// Heuristic: Commit Message (Fallback if requested separately)
	if strings.Contains(lowerPrompt, "commit message") {
		return "feat: Implement primes.py", nil
	}

	// Default Response
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
