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

	// 1. Heuristics for "generate-from-spec" (Technical Program Manager)
	if strings.Contains(prompt, "Output purely JSON") || strings.Contains(prompt, "Technical Program Manager") {
		// Return a mock Jira ticket plan JSON
		return `{
  "epics": [
    {
      "title": "ID:[PRIMES] Prime Number Script",
      "description": "Implement a script to calculate prime numbers.",
      "stories": [
        {
          "title": "Core Calculation Logic",
          "description": "Implement the Sieve of Eratosthenes."
        }
      ]
    }
  ]
}`, nil
	}

	// 2. Heuristics for "Initializer Agent" (Bash Script)
	if strings.Contains(prompt, "Initializer Agent") {
		return "```bash\n# Initialize\necho 'Initializing workspace'\n```", nil
	}

	// 3. Heuristics for "Prime Number Script" (The Coding Task)
	if strings.Contains(prompt, "ID:[PRIMES]") {
		// Return Python code for primes
		return "```python\ndef is_prime(n):\n    if n <= 1: return False\n    for i in range(2, int(n**0.5)+1):\n        if n % i == 0: return False\n    return True\n\nprint([x for x in range(20) if is_prime(x)])\n```", nil
	}

	// 4. Heuristics for "QA AGENT" (Verification)
	if strings.Contains(prompt, "QA AGENT") {
		return "```bash\nrecac-agent signal QA_PASSED\n```", nil
	}

	// 5. Heuristics for "Manager Agent" (Sign-off)
	if strings.Contains(prompt, "Manager Agent") {
		return "```bash\nrecac-agent signal PROJECT_SIGNED_OFF\n```", nil
	}

	// 6. Generic "Agent" task completion (e.g. implementation)
	// If it asks to implement something, we just mark it as done or return dummy code.
	if strings.Contains(prompt, "You are an Agent") {
		// Mock updating the feature status if we can infer it, or just say done.
		return "Task completed. ```bash\necho 'Work done'\n```", nil
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
