package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"recac/internal/db"
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

	// Heuristics for E2E scenarios
	lowerPrompt := strings.ToLower(prompt)

	// 1. Completion Heuristic (prevents no-op loop)
	if strings.Contains(lowerPrompt, "nothing to commit") {
		return "The task appears to be completed. I will verify the changes.", nil
	}

	// 2. Initializer Heuristic (must be prioritized over coding for feature list request)
	if strings.Contains(prompt, "INITIALIZER") {
		features := []db.Feature{
			{ID: "PRIMES", Description: "Calculate prime numbers up to 10,000", Status: "todo"},
		}
		fl := db.FeatureList{
			ProjectName: "Mock Project",
			Features:    features,
		}
		data, _ := json.Marshal(fl)
		return fmt.Sprintf("```json\n%s\n```", string(data)), nil
	}

	// 3. Coding Agent Heuristic (Prime Number Scenario)
	if strings.Contains(lowerPrompt, "prime number") || strings.Contains(lowerPrompt, "primes.json") || strings.Contains(prompt, "ID:[PRIMES]") {
		// Return a python script to solve the task
		return `I will create a python script to calculate prime numbers and save them to primes.json.

` + "```python" + `
import json

def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

primes = [n for n in range(2, 10000) if is_prime(n)]

with open("primes.json", "w") as f:
    json.dump({"primes": primes}, f)

print(f"Generated {len(primes)} primes")
` + "```" + `

I will also mark the feature as passed using agent-bridge.

` + "```bash" + `
agent-bridge feature set PRIMES --status done --passes true
` + "```", nil
	}

	// 4. QA Agent Heuristic
	if strings.Contains(prompt, "QA AGENT") {
		return "PASS", nil
	}

	// 5. Manager Agent Heuristic
	if strings.Contains(prompt, "PROJECT MANAGER") {
		return `I have reviewed the progress.
The feature "Calculate prime numbers" is marked as done.
I am creating the sign-off signal.

` + "```bash" + `
agent-bridge signal create PROJECT_SIGNED_OFF true
` + "```" + `

Approved.`, nil
	}

	// Default fallback
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
