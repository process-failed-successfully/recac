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
// It returns a mock response based on prompt heuristics
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	lowerPrompt := strings.ToLower(prompt)

	// Heuristic for Architect/TPM/Initializer (Planning phase)
	// Must return a JSON array of ticketNode objects
	// Prioritize this to prevent returning code during planning
	if (strings.Contains(lowerPrompt, "json") && (strings.Contains(lowerPrompt, "architect") || strings.Contains(lowerPrompt, "technical program manager") || strings.Contains(lowerPrompt, "create exactly one ticket"))) {
		tickets := []map[string]interface{}{
			{
				"summary": "ID:[PRIMES] Implement Python Script",
				"description": "Write a Python script that calculates prime numbers up to 100 and writes them to a file named 'primes.json'. ensure correct json format.",
				"type": "Story",
				"children": []map[string]interface{}{},
			},
		}
		jsonBytes, _ := json.Marshal(tickets)
		return fmt.Sprintf("```json\n%s\n```", string(jsonBytes)), nil
	}

	// Heuristic for Coding (Execution phase) - Python Primes
	if strings.Contains(lowerPrompt, "prime") && strings.Contains(lowerPrompt, "python") {
		code := `
import json

def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

primes = [x for x in range(101) if is_prime(x)]

with open("primes.json", "w") as f:
    json.dump(primes, f)
print("Primes written to primes.json")
`
		return fmt.Sprintf("Here is the python script:\n```python\n%s\n```", code), nil
	}

	// Fallback
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
