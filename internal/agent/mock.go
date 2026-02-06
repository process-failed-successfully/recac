package agent

import (
	"context"
	"fmt"
	"strings"
)

// MockAgent is a smart mock agent for testing and mock mode
// It includes heuristics to pass E2E scenarios like prime-python
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
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// Heuristics for Smoke Tests (prime-python)

	// 0. Initializer (Feature List Creation)
	if strings.Contains(prompt, "INITIALIZER AGENT") {
		return `
I will set up the project and create the feature list.

` + "```bash" + `
cat << 'EOF' | agent-bridge import
{
  "features": [
    {
      "id": "req-must-correctly-identify-prime-",
      "category": "functional",
      "description": "Must correctly identify prime numbers",
      "status": "pending",
      "steps": ["Verify is_prime(5) returns True", "Verify is_prime(4) returns False"],
      "priority": "MVP",
      "passes": false,
      "dependencies": {
          "exclusive_write_paths": ["primes.py"],
          "read_only_paths": []
      }
    },
    {
      "id": "req-must-print-primes-up-to-20",
      "category": "functional",
      "description": "Must print primes up to 20",
      "status": "pending",
      "steps": ["Run script and check output"],
      "priority": "MVP",
      "passes": false,
      "dependencies": {
          "exclusive_write_paths": ["primes.py"],
          "read_only_paths": []
      }
    }
  ]
}
EOF

echo "Project initialized."
` + "```" + `
`, nil
	}

	// 1. Ticket Generation (TPM Agent)
	// Check this FIRST to avoid confusion with implementation keywords that might appear in the spec.
	// STRICTER CHECK: Must be Technical Program Manager AND contain "tickets" or "ID:[PRIMES]"
	// This prevents false positives where the Coding Agent prompt mentions "tickets".
	if strings.Contains(prompt, "Technical Program Manager") && (strings.Contains(prompt, "tickets") || strings.Contains(prompt, "ID:[PRIMES]")) {
		return `
[
  {
    "title": "ID:[PRIMES] Implement Prime Number Calculator",
    "description": "Implement a Python script to calculate prime numbers.",
    "type": "Story",
    "acceptance_criteria": [
      "Must correctly identify prime numbers",
      "Must print primes up to 20"
    ],
    "children": []
  }
]
`, nil
	}

	// 2. QA / Verification Phase
	if strings.Contains(prompt, "YOUR ROLE - QA AGENT") {
		return "```bash\nagent-bridge signal QA_PASSED true\n```\nQA Passed.", nil
	}

	// 3. Manager Sign-off
	if strings.Contains(prompt, "PROJECT MANAGER") {
		// If prompt indicates pending/incomplete features, DO NOT sign off.
		// Instead, instruct to proceed with implementation.
		lowerPrompt := strings.ToLower(prompt)
		if strings.Contains(lowerPrompt, "pending") || strings.Contains(lowerPrompt, "incomplete") || strings.Contains(lowerPrompt, "passes: false") {
			return "Project is not ready for sign-off. Please complete the pending features.", nil
		}
		return "```bash\nagent-bridge signal PROJECT_SIGNED_OFF true\n```\nProject Approved.", nil
	}

	// 4. Implementation Phase (primes.py)
	// Case-insensitive check for robustness
	promptLower := strings.ToLower(prompt)
	if strings.Contains(promptLower, "calculate primes") || strings.Contains(prompt, "[PRIMES]") || strings.Contains(promptLower, "prime numbers") || strings.Contains(prompt, "req-must-correctly-identify-prime-") {
		return `
Sure, I will create a python script to calculate primes.

` + "```bash" + `
cat <<EOF > primes.py
def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

print([x for x in range(20) if is_prime(x)])
EOF

# Signal feature completion
agent-bridge feature set req-must-correctly-identify-prime- --status done --passes true
agent-bridge feature set req-must-print-primes-up-to-20 --status done --passes true
` + "```" + `
`, nil
	}

	// Default response
	// Includes a dummy command to prevent "NO-OP LOOP" circuit breaker
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...\n\n```bash\necho \"Mock agent received prompt\"\n```",
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
