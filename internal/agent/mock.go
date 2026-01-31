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
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// Smart Mock Logic for Smoke Tests

	// 1. Ticket Generation Request (Prime Python Scenario - TPM Phase)
	if strings.Contains(prompt, "Technical Program Manager") || (strings.Contains(prompt, "ID:[PRIMES]") && strings.Contains(prompt, "JSON format")) {
		return `[
  {
    "title": "[GEN] Create Prime Number Script",
    "description": "Create a python script named 'primes.py' that calculates primes < 10000 and outputs to 'primes.json'. ID:[PRIMES]",
    "type": "Task",
    "children": []
  }
]`, nil
	}

	// 2. Initializer Request (Create feature list)
	if strings.Contains(prompt, "Create feature_list.json") {
		return `I will create the feature_list.json for this task.

` + "```bash" + `
agent-bridge import <<EOF
{
  "project_name": "MFLP-2408",
  "features": [
    {
      "description": "Create a python script named 'primes.py' that calculates primes < 10000",
      "status": "pending",
      "passes": false
    },
    {
      "description": "Output primes to 'primes.json'",
      "status": "pending",
      "passes": false
    }
  ]
}
EOF
` + "```" + `
`, nil
	}

	// 3. Implementation Request (Writing the file)
	// Matches prompt asking to implement "PRIMES", "primes.py", or just "prime number"
	lowerPrompt := strings.ToLower(prompt)
	if strings.Contains(prompt, "PRIMES") || strings.Contains(prompt, "primes.py") || strings.Contains(lowerPrompt, "prime number") {
		return `I will create the primes.py script and the json output as requested.

` + "```bash" + `
# Create the python script
cat << 'EOF' > primes.py
import json

def get_primes(n):
    primes = []
    for num in range(2, n):
        is_prime = True
        for i in range(2, int(num ** 0.5) + 1):
            if num % i == 0:
                is_prime = False
                break
        if is_prime:
            primes.append(num)
    return primes

primes_list = get_primes(10000)
with open("primes.json", "w") as f:
    json.dump({"primes": primes_list}, f)
EOF

# Run the script to generate the json
python3 primes.py

# Add and commit
git add primes.py primes.json || true
git commit -m "Add primes script and output" || true
` + "```" + `
`, nil
	}

	// 4. Completion Signal (All features done)
	if strings.Contains(prompt, "All features are marked as done") || strings.Contains(prompt, "signal completion") {
		return `It looks like all features are implemented. I will signal completion.

` + "```bash" + `
agent-bridge signal COMPLETED true
` + "```" + `
`, nil
	}

	// 5. QA Agent Request
	if strings.Contains(prompt, "QA AGENT") {
		return `I have verified the implementation. All features are working correctly.

` + "```bash" + `
# Signal QA passed
agent-bridge signal QA_PASSED true
` + "```" + `
`, nil
	}

	// 6. Manager Review Request
	if strings.Contains(prompt, "QA Report") || strings.Contains(prompt, "Manager") {
		return `I have reviewed the work and it looks good. Signing off.

` + "```bash" + `
agent-bridge signal PROJECT_SIGNED_OFF true
` + "```" + `
`, nil
	}

	// 7. UI Verification Request
	if strings.Contains(prompt, "UI VERIFICATION AGENT") || strings.Contains(prompt, "ui_verification.json") {
		return `I have verified the UI changes.

` + "```bash" + `
# Signal UI Verification passed
agent-bridge signal UI_VERIFIED true
` + "```" + `
`, nil
	}

	// Default Mock Response
	// We include a no-op bash block to ensure the executor doesn't trip the "no commands" circuit breaker
	// Note: We avoid "echo" with quotes in the preview to prevent confusion with the security scanner's string masking logic in tests
	response := fmt.Sprintf("%s:\n\nI received your prompt. In mock mode, I would process this request.\n\nPrompt snippet: %.50s...\n\n```bash\n# no-op to prevent circuit breaker\necho 'mock agent alive'\n```",
		m.responsePrefix, prompt)
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
