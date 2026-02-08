package agent

import (
	"context"
	"fmt"
	"os"
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

	// Heuristic: Initializer Agent (Feature Generation)
	// Must come before generic "app_spec.txt" check as this prompt also contains it.
	if strings.Contains(prompt, "## YOUR ROLE - INITIALIZER AGENT") {
		return `I will initialize the project with the requested features.

` + "```bash" + `
cat << 'EOF' | agent-bridge import
{
  "project_name": "Primes Calculator",
  "features": [
    {
      "id": "req-script-prints-primes-up-to-100",
      "category": "functional",
      "priority": "MVP",
      "description": "[PRIMES] Script calculates primes up to 10000",
      "status": "pending",
      "steps": [
        "Run python3 primes.py",
        "Verify output contains prime numbers up to 10000"
      ],
      "passes": false,
      "dependencies": {
        "depends_on_ids": [],
        "exclusive_write_paths": ["primes.py"],
        "read_only_paths": []
      }
    },
    {
      "id": "req-script-is-runnable",
      "category": "functional",
      "priority": "MVP",
      "description": "Script is runnable",
      "status": "pending",
      "steps": [
        "Run python3 primes.py",
        "Verify exit code is 0"
      ],
      "passes": false,
      "dependencies": {
        "depends_on_ids": [],
        "exclusive_write_paths": ["primes.py"],
        "read_only_paths": []
      }
    }
  ]
}
EOF
` + "```" + `
`, nil
	}

	// Heuristic: Detect ticket generation prompt
	// The prompt often contains "app_spec.txt" or identifies as "Technical Program Manager"
	// We must be strict here to avoid matching "Coding Agent" prompts that mention app_spec.txt
	if strings.Contains(prompt, "Technical Program Manager") && (strings.Contains(prompt, "tickets") || strings.Contains(prompt, "plan") || strings.Contains(prompt, "Specification") || strings.Contains(prompt, "app_spec.txt")) {
		return `[
  {
    "title": "ID:[PRIMES] Implement Primes Calculation",
    "description": "Create a Python script to calculate prime numbers.",
    "type": "Story",
    "acceptance_criteria": [
      "Script prints primes up to 10000",
      "Script is runnable"
    ]
  }
]`, nil
	}

	// Heuristic: Project Manager Approval (Smoke Test)
	if strings.Contains(prompt, "PROJECT MANAGER") && strings.Contains(prompt, "Approve or Reject") {
		return `I have reviewed the work and it meets the requirements.

` + "```bash" + `
agent-bridge signal PROJECT_SIGNED_OFF true --privileged
` + "```" + `
`, nil
	}

	// Heuristic: Detect Primes Implementation Task
	// This supports the E2E smoke test scenario
	// We check the prompt AND the injected features env var (for robustness)
	injectedFeatures := os.Getenv("RECAC_INJECTED_FEATURES")
	// Debug logging for troubleshooting CI - use stderr to ensure it appears in logs
	fmt.Fprintf(os.Stderr, "[DEBUG] MockAgent: injectedFeatures=%q\n", injectedFeatures)
	fmt.Fprintf(os.Stderr, "[DEBUG] MockAgent: prompt_preview=%q\n", truncateString(prompt, 100))

	// Robust check for Primes scenario
	if strings.Contains(prompt, "[PRIMES]") ||
		strings.Contains(prompt, "primes.py") ||
		strings.Contains(injectedFeatures, "[PRIMES]") ||
		strings.Contains(injectedFeatures, "req-script-prints-primes") {
		return `I will implement the primes calculation script as requested.

` + "```bash" + `
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n < 2: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = [x for x in range(10000) if is_prime(x)]
print(f"Calculated {len(primes)} primes")
with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
EOF

python3 primes.py
agent-bridge feature set req-script-prints-primes-up-to-100 passed
agent-bridge feature set req-script-is-runnable passed
` + "```" + `
`, nil
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
