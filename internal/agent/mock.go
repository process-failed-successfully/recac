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

	// Heuristic: Detect Primes Implementation Task (Coding Agent)
	// This supports the E2E smoke test scenario. We prioritize this over TPM if it looks like a coding task.
	// We check for "Coding Agent", "Developer", "primes.py", or the specific ID tag.
	// CRITICAL: We must EXCLUDE "Technical Program Manager" or "app_spec.txt" to prevent false positives
	// when the TPM prompt contains the spec (which includes "[PRIMES]" and "primes.py").
	// We also check for lowercase "primes" as the task description might use it (e.g. "Script prints primes up to 100").
	if (strings.Contains(prompt, "[PRIMES]") || strings.Contains(prompt, "primes.py") || strings.Contains(prompt, "primes")) &&
		(strings.Contains(prompt, "Coding Agent") || strings.Contains(prompt, "CODING AGENT") || strings.Contains(prompt, "Developer") || strings.Contains(prompt, "primes.py")) &&
		!strings.Contains(prompt, "Technical Program Manager") {
		return `I will implement the primes calculation script as requested.

` + "```bash" + `
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n < 2: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = [x for x in range(101) if is_prime(x)]
print(primes)
with open('primes.json', 'w') as f:
    json.dump(primes, f)
EOF

python3 primes.py
agent-bridge feature set req-script-prints-primes-up-to-100 --status done --passes true
agent-bridge feature set req-script-is-runnable --status done --passes true
` + "```" + `
`, nil
	}

	// Heuristic: Detect ticket generation prompt (TPM)
	// The prompt often contains "app_spec.txt" or identifies as "Technical Program Manager"
	// We check this AFTER the coding agent check to avoid false positives from history
	if strings.Contains(prompt, "app_spec.txt") || strings.Contains(prompt, "tickets") || strings.Contains(prompt, "Technical Program Manager") {
		return `[
  {
    "title": "ID:[PRIMES] Implement Primes Calculation",
    "description": "Create a Python script to calculate prime numbers.",
    "type": "Story",
    "acceptance_criteria": [
      "Script prints primes up to 100",
      "Script is runnable"
    ]
  }
]`, nil
	}

	// Return a mock response that shows the agent received the prompt
	// This allows the session to run without requiring real API keys
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100))

	// Append a dummy command to prevent NO-OP loop detection in the runner
	// Note: We use triple backticks here which should be matched by bashBlockRegex
	response += "\n\nI will execute a dummy command to signal liveness:\n```bash\necho \"Mock Agent is alive\"\n```"

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
