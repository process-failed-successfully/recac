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
// It uses heuristics to return appropriate responses based on the prompt content
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// Heuristic 1: Technical Program Manager (Planning Phase)
	// Expected Output: JSON Feature List
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "ticket plan") {
		return `{
  "project_name": "mock-project",
  "features": [
    {
      "id": "PRIMES",
      "category": "Core",
      "priority": "High",
      "description": "Implement a python script that prints prime numbers",
      "status": "Pending",
      "steps": ["Create primes.py", "Implement is_prime function", "Add main loop"],
      "dependencies": {
        "depends_on_ids": [],
        "exclusive_write_paths": [],
        "read_only_paths": []
      }
    }
  ]
}`, nil
	}

	// Heuristic 2: Coding Agent (Implementation Phase)
	// Expected Output: Code blocks
	if strings.Contains(prompt, "Coding Agent") || strings.Contains(prompt, "Implement") {
		return `I will implement the requested feature.

'''bash
echo "Creating primes.py"
cat <<EOF > primes.py
def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

if __name__ == "__main__":
    for i in range(20):
        if is_prime(i):
            print(i)
EOF
'''
`, nil
	}

	// Default Fallback
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
