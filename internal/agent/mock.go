package agent

import (
	"context"
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

	// 1. Planning Prompt (generate-from-spec)
	// We need to return valid JSON for the ticket plan
	if containsKeyword(prompt, "Technical Program Manager") || containsKeyword(prompt, "generate-from-spec") {
		return `[
    {
      "title": "Implement Prime Number Checker",
      "description": "Create a python script that checks for prime numbers.",
      "type": "Task",
      "id": "[PRIMES]",
      "blocked_by": []
    }
]`, nil
	}

	// 2. Initializer Prompt (Bridge Import)
	// Must return a bash block invoking agent-bridge import to load features
	if containsKeyword(prompt, "agent-bridge import") {
		return "```bash\ncat <<EOF > feature_list.json\n{\n  \"project_name\": \"mock-project\",\n  \"features\": [\n    {\n      \"id\": \"[PRIMES]\",\n      \"name\": \"Implement Prime Number Checker\",\n      \"status\": \"todo\",\n      \"passes\": false\n    }\n  ]\n}\nEOF\n\ncat feature_list.json | agent-bridge import\n```", nil
	}

	// 3. Coding/Execution Prompt
	// If it looks like a coding task (e.g. mentions primes.py or the ID), return a bash script
	if containsKeyword(prompt, "primes.py") || containsKeyword(prompt, "req-primes") {
		return "```bash\ncat <<EOF > primes.py\ndef is_prime(n):\n    if n <= 1:\n        return False\n    for i in range(2, int(n**0.5) + 1):\n        if n % i == 0:\n            return False\n    return True\n\nif __name__ == '__main__':\n    import sys\n    print(is_prime(int(sys.argv[1])))\nEOF\n\n# Run it to verify\npython3 primes.py 7\n\n# Commit\ngit add primes.py\ngit commit -m \"Add prime checker\"\n\n# Mark done\nagent-bridge feature set \"[PRIMES]\" --status done --passes true\n```", nil
	}

	// Default: Return a no-op bash block to satisfy the runner's loop
	// without causing an error or infinite loop of text.
	return "```bash\n# no-op\necho \"Mock Agent: No specific action for this prompt.\"\n```", nil
}

func containsKeyword(s, sub string) bool {
	// Simple containment check
	// In a real scenario, we might want case-insensitive
	// but for now, exact match on known prompt templates is enough
	// or specific keywords
	return len(s) >= len(sub) && (s == sub || search(s, sub))
}

func search(haystack, needle string) bool {
	// naive search
	for i := 0; i <= len(haystack)-len(needle); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
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
