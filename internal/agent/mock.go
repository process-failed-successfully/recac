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

	promptLower := strings.ToLower(prompt)

	// Heuristic: If looking for "technical program manager" and expecting JSON (TPM phase)
	if strings.Contains(promptLower, "technical program manager") && strings.Contains(promptLower, "json") {
		return `[
  {
    "id": "PRIMES",
    "type": "Story",
    "title": "Implement Python Script",
    "description": "Create a Python script to generate primes.",
    "dependencies": [],
    "priority": "High",
    "file_paths": ["primes.py"]
  }
]`, nil
	}

	// Heuristic: If coding phase (e.g. primes)
	if strings.Contains(promptLower, "prime number") || strings.Contains(promptLower, "primes.json") {
		return "```python\nimport json\n\nprimes = [2, 3, 5, 7, 11, 13, 17, 19, 23, 29]\nwith open('primes.json', 'w') as f:\n    json.dump({\"primes\": primes}, f)\n```", nil
	}

	// Default generic response
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
