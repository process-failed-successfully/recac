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

	// Normalize prompt for heuristics
	lowerPrompt := strings.ToLower(prompt)

	// Heuristic 1: Ticket Generation (TPM/Architect)
	if (strings.Contains(lowerPrompt, "technical program manager") || strings.Contains(lowerPrompt, "architect")) && strings.Contains(lowerPrompt, "json") {
		// Return a mock ticket structure
		return `[
			{
				"id": "PRIMES",
				"type": "Task",
				"summary": "Implement Python Script",
				"description": "Create primes.py that calculates primes < 10000 and outputs to primes.json.",
				"status": "To Do",
				"children": []
			}
		]`, nil
	}

	// Heuristic 2: Coding Task (Agent)
	if strings.Contains(lowerPrompt, "primes") || strings.Contains(lowerPrompt, "python") {
		// Return a mock implementation command
		return "I will implement the prime number script.\n\n```bash\ncat << 'EOF' > primes.py\nprimes = []\nfor num in range(2, 10000):\n    is_prime = True\n    for i in range(2, int(num ** 0.5) + 1):\n        if num % i == 0:\n            is_prime = False\n            break\n    if is_prime:\n        primes.append(num)\n\nimport json\nwith open('primes.json', 'w') as f:\n    json.dump({'primes': primes}, f)\nEOF\n\npython3 primes.py\ngit add primes.py primes.json\ngit commit -m \"Add primes script\"\n```", nil
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
