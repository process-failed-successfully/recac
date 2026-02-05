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

	// Heuristics for E2E Smoke Tests

	// 1. Initializer Agent
	if strings.Contains(prompt, "INITIALIZER AGENT") {
		return m.initializerResponse(), nil
	}

	// 2. Technical Program Manager (TPM) - Ticket Generation
	if strings.Contains(prompt, "Technical Program Manager") {
		return m.tpmResponse(prompt), nil
	}

	// 3. Project Manager (Check before QA to prevent infinite loops if context has QA_PASSED)
	// The prompt often contains "Manager" or asks for sign off
	if strings.Contains(prompt, "PROJECT MANAGER") || strings.Contains(prompt, "Manager") {
		// Heuristic: If we are asked to review and sign off
		if strings.Contains(prompt, "Signal PROJECT_SIGNED_OFF") || strings.Contains(prompt, "sign off") {
			return m.managerResponse(), nil
		}
	}

	// 4. QA Agent
	if strings.Contains(prompt, "QA AGENT") {
		return m.qaResponse(), nil
	}

	// 5. Developer / Coding Agent (Primes Scenario)
	if strings.Contains(prompt, "primes.py") || strings.Contains(prompt, "[PRIMES]") {
		return m.primesDeveloperResponse(), nil
	}

	// Fallback generic response
	// We include a dummy command to prevent the "NO-OP LOOP" circuit breaker from tripping
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...\n\n```bash\necho \"Mock Agent: Processing %s\"\n```",
		m.responsePrefix, len(prompt), truncateString(prompt, 100), truncateString(prompt, 20))
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

func (m *MockAgent) initializerResponse() string {
	// Returns a script to import the feature list
	return `
cat << 'EOF' | agent-bridge import
{
  "features": [
    {"id": "req-primes-py-exists", "description": "primes.py exists", "status": "pending"},
    {"id": "req-primes-json-exists", "description": "primes.json exists", "status": "pending"}
  ]
}
EOF
`
}

func (m *MockAgent) tpmResponse(prompt string) string {
	// If it's the specific primes scenario
	if strings.Contains(prompt, "[PRIMES]") || strings.Contains(prompt, "primes.py") {
		return `{
  "tickets": [
    {
      "id": "TASK-1",
      "title": "Create primes.py",
      "description": "Create a python script that calculates primes and writes them to primes.json. [PRIMES]",
      "type": "task",
      "assigned_to": "coding_agent",
      "status": "todo"
    }
  ]
}`
	}

	// Default generic plan
	return `{
  "tickets": [
    {
      "id": "EPIC-1",
      "title": "Project Setup",
      "description": "Initial project setup",
      "type": "epic",
      "assigned_to": "architect_agent",
      "status": "todo"
    }
  ]
}`
}

func (m *MockAgent) primesDeveloperResponse() string {
	// Returns bash script to implement the feature
	// Must use markdown code blocks as per memory guidelines?
	// The prompt usually expects a response with blocks.
	// But let's check the memory: "MockAgent ... must use standard Markdown code block delimiters"

	return "```bash\n" + `
echo "Creating primes.py..."
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n < 2: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = [x for x in range(1, 101) if is_prime(x)]
with open("primes.json", "w") as f:
    json.dump(primes, f)
print(f"Generated {len(primes)} primes")
EOF

# Run it to generate the json
python3 primes.py

# Signal features passed
agent-bridge feature set req-primes-py-exists passed
agent-bridge feature set req-primes-json-exists passed
` + "\n```"
}

func (m *MockAgent) qaResponse() string {
	return "```bash\n" + `
echo "QA Checks Passed"
agent-bridge signal QA_PASSED
` + "\n```"
}

func (m *MockAgent) managerResponse() string {
	return "```bash\n" + `
echo "Project Sign Off"
agent-bridge signal PROJECT_SIGNED_OFF
` + "\n```"
}

// truncateString truncates a string to a maximum length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
