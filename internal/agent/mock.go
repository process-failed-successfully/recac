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

	// 1. TPM Role (Planning)
	if strings.Contains(prompt, "You are an expert Technical Program Manager") {
		return "```json\n[\n  {\n    \"title\": \"ID:[PRIMES] Implement Primes\",\n    \"description\": \"Implement a Python script to check for prime numbers.\",\n    \"priority\": \"High\",\n    \"type\": \"Task\",\n    \"dependencies\": []\n  }\n]\n```", nil
	}

	// 2. Initializer Role (Setup Features)
	if strings.Contains(prompt, "INITIALIZER AGENT") {
		return `Here is the initialization script:

` + "```bash" + `
#!/bin/bash
cat << 'EOF' | agent-bridge import
{
  "features": [
    {
      "id": "req-must-correctly-identify-prime-",
      "description": "Must correctly identify prime numbers",
      "priority": "critical",
      "status": "pending"
    },
    {
      "id": "req-must-be-a-valid-python-script",
      "description": "Must be a valid Python script",
      "priority": "critical",
      "status": "pending"
    }
  ]
}
EOF
` + "```" + `
`, nil
	}

	// 3. Coding Role (Implementation)
	// We check for [PRIMES] tag or role header or file name
	if strings.Contains(prompt, "CODING AGENT") || strings.Contains(prompt, "[PRIMES]") || strings.Contains(prompt, "primes.py") {
		return `Here is the implementation:

` + "```bash" + `
#!/bin/bash
# Create primes.py
cat << 'EOF' > primes.py
def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

if __name__ == "__main__":
    import sys
    if len(sys.argv) > 1:
        n = int(sys.argv[1])
        print(is_prime(n))
EOF

# Run it to verify
python3 primes.py 7

# Update features
agent-bridge feature update req-must-correctly-identify-prime- --status completed
agent-bridge feature update req-must-be-a-valid-python-script --status completed

# Signal completion
agent-bridge signal COMPLETED true
` + "```" + `
`, nil
	}

	// 4. QA Role (Verification)
	if strings.Contains(prompt, "QA AGENT") {
		return "Tests passed.\n\n```bash\nagent-bridge signal QA_PASSED true\n```", nil
	}

	// 5. Manager Role (Sign-off)
	if strings.Contains(prompt, "PROJECT MANAGER") {
		return "Project approved.\n\n```bash\nagent-bridge signal PROJECT_SIGNED_OFF true\n```", nil
	}

	// Return a mock response that shows the agent received the prompt
	// This allows the session to run without requiring real API keys
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
