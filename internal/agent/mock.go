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
// It returns a mock response based on heuristics
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	// 1. Completion Heuristic (Must be first to prevent NO-OP LOOP)
	if strings.Contains(strings.ToLower(prompt), "nothing to commit") {
		return `{"action": "commit", "message": "No changes to commit"}`, nil
	}

	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	lowerPrompt := strings.ToLower(prompt)

	// 2. Initializer Agent Heuristic (Prioritized over Coding)
	if strings.Contains(lowerPrompt, "initializer") || strings.Contains(lowerPrompt, "create a plan") {
		response := map[string]interface{}{
			"plan": []map[string]string{
				{
					"id":          "task-1",
					"description": "Implement the requested feature",
					"file":        "main.py",
				},
			},
			"summary": "Plan created",
		}
		bytes, _ := json.Marshal(response)
		return string(bytes), nil
	}

	// 3. TPM Agent Heuristic
	if strings.Contains(lowerPrompt, "tpm") || strings.Contains(lowerPrompt, "feature breakdown") {
		response := map[string]interface{}{
			"features": []map[string]string{
				{
					"id":          "feat-1",
					"description": "Core functionality",
					"status":      "pending",
				},
			},
		}
		bytes, _ := json.Marshal(response)
		return string(bytes), nil
	}

	// 4. QA Agent Heuristic
	if strings.Contains(lowerPrompt, "qa agent") || strings.Contains(lowerPrompt, "verify") {
		// QA passes
		return "agent-bridge signal --success --message 'Verification passed'", nil
	}

	// 5. Manager Agent Heuristic
	if strings.Contains(lowerPrompt, "manager agent") || strings.Contains(lowerPrompt, "review") {
		// Manager approves
		return "agent-bridge signal --approve --message 'Approved'", nil
	}

	// 6. Coding Agent Heuristic (Primes Scenario)
	if strings.Contains(lowerPrompt, "prime number") ||
	   strings.Contains(lowerPrompt, "primes.json") ||
	   strings.Contains(lowerPrompt, "id:[primes]") ||
	   strings.Contains(lowerPrompt, "generate primes") {

		pythonScript := `
import json

def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

primes = [i for i in range(10001) if is_prime(i)]

with open("primes.json", "w") as f:
    json.dump({"primes": primes}, f)

print(f"Generated {len(primes)} primes")
`
		// Wrap in markdown code block
		return fmt.Sprintf("Here is the python script:\n```python\n%s\n```", pythonScript), nil
	}

	// Default Mock Response
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
