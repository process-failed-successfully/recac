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

	// Detect Prime Python Scenario
	if strings.Contains(prompt, "primes.py") || strings.Contains(prompt, "prime numbers") {
		// Differentiate between Planning (Ticket Generation) and Implementation
		if strings.Contains(prompt, "CRITICAL INSTRUCTION FOR TICKET GENERATION") {
			return m.generatePrimesTickets(), nil
		}
		return m.generatePrimesResponse(), nil
	}

	// Detect QA Agent Role
	if strings.Contains(prompt, "YOUR ROLE - QA AGENT") {
		return m.generateQAPassedResponse(), nil
	}

	// Detect Manager Agent Role
	if strings.Contains(prompt, "PROJECT MANAGER") {
		return m.generateManagerSignOffResponse(), nil
	}

	// Default response
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100))
	return response, nil
}

func (m *MockAgent) generateQAPassedResponse() string {
	return `I have verified the project and all tests pass.

` + "```bash" + `
# Run tests (simulated)
echo "Running tests..."
# Signal QA Passed
agent-bridge signal QA_PASSED true
` + "```" + `
`
}

func (m *MockAgent) generateManagerSignOffResponse() string {
	return `I have reviewed the QA report and the project implementation. Everything looks good.

` + "```bash" + `
# Sign off on the project
agent-bridge signal PROJECT_SIGNED_OFF true
` + "```" + `
`
}

func (m *MockAgent) generatePrimesTickets() string {
	// IMPORTANT: The format must explicitly match what the runner expects.
	// The logs showed a "You must specify a summary of the issue" error, which implies the runner might be
	// sending separate API calls or the JSON structure for creating tickets is being parsed/used incorrectly
	// if it's just a raw list.
	// However, usually the agent output is parsed by `GenerateTickets` which expects a JSON list of objects.
	// The AppSpec says: "CRITICAL INSTRUCTION: You MUST create exactly ONE ticket. Type: Task."
	// Let's refine the summary/description to match the spec exactly to be safe.
	return `[
  {
    "title": "ID:[PRIMES] Create Prime Number Script",
    "type": "Task",
    "description": "Create a python script named 'primes.py'. It MUST be python.\nIt must calculate all prime numbers less than 10,000 and output to a file named 'primes.json'.\nIMPORTANT: You MUST use a bash block to create the file.",
    "labels": ["recac-smoke-test"]
  }
]`
}

func (m *MockAgent) generatePrimesResponse() string {
	primesContent := `import json

primes = []
for num in range(2, 10000):
    is_prime = True
    for i in range(2, int(num ** 0.5) + 1):
        if num % i == 0:
            is_prime = False
            break
    if is_prime:
        primes.append(num)

with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
`

	// We wrap the python content in a bash block to write it
	// We also commit it and signal completion
	return fmt.Sprintf(`I will implement the prime number script as requested.

` + "```bash" + `
# Create the python script
cat << 'EOF' > primes.py
%s
EOF

# Run the script to generate output
python3 primes.py

# Commit the files
git add primes.py primes.json
git commit -m "Add primes.py and primes.json" || echo "Nothing to commit"

# Signal completion
agent-bridge signal COMPLETED true
` + "```" + `
`, primesContent)
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
