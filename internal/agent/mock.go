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

	// 1. Detect Ticket Generation (TPM Role)
	// The prompt typically contains "Technical Program Manager" or "ticket generation"
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "tickets") {
		// Return a hardcoded JSON array of tickets for the prime-python scenario
		// This mimics the expected output for the E2E test
		return `[
  {
    "title": "ID:[PRIMES] Create Python script to calculate primes",
    "description": "Develop a Python script that calculates prime numbers up to a specified limit. The script should be efficient and well-documented.",
    "type": "Story",
    "priority": "High",
    "status": "To Do",
    "labels": ["backend", "python"]
  }
]`, nil
	}

	// 2. Detect Implementation Request (Software Engineer Role)
	// The prompt might ask to "calculate primes" or mention the ID
	if strings.Contains(prompt, "calculate primes") || strings.Contains(prompt, "[PRIMES]") {
		return "```bash\n# Create the python script\necho 'def is_prime(n):\n    if n <= 1: return False\n    for i in range(2, int(n**0.5) + 1):\n        if n % i == 0: return False\n    return True\n\nimport sys\n\ndef main():\n    limit = 100\n    if len(sys.argv) > 1:\n        limit = int(sys.argv[1])\n    primes = [i for i in range(2, limit + 1) if is_prime(i)]\n    print(f\"Primes up to {limit}: {primes}\")\n\nif __name__ == \"__main__\":\n    main()' > primes.py\n```", nil
	}

	// 3. Detect QA/Verification Request
	if strings.Contains(prompt, "QA") || strings.Contains(prompt, "verify") {
		return "I have verified the implementation. It works as expected.", nil
	}

	// Default response for unknown prompts
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
