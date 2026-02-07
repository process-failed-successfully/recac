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

	// 1. Detect Ticket Generation (TPM Agent)
	// The prompt usually contains "Technical Program Manager" or "spec"
	// and specific keywords from the scenario.
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "TPM") {
		if strings.Contains(prompt, "[PRIMES]") || strings.Contains(prompt, "Prime Number Script") {
			// Return JSON for prime-python scenario
			return `
[
  {
    "title": "ID:[PRIMES] Create Prime Number Script",
    "description": "Implement a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to a file named 'primes.json'.\n\nRepo: https://github.com/process-failed-successfully/recac-jira-e2e",
    "type": "Task",
    "acceptance_criteria": [
      "primes.py exists",
      "primes.json exists and contains valid JSON",
      "primes.json contains exactly 1229 primes"
    ],
    "children": []
  }
]`, nil
		}
	}

	// 2. Detect Initializer Agent
	// Memory says: "The MockAgent ... must detect 'Initializer' role prompts..."
	if strings.Contains(prompt, "Initializer") || strings.Contains(prompt, "initialize the repository") {
		return "```bash\ngit init\ngit config user.name \"Recac Agent\"\ngit config user.email \"agent@recac.ai\"\nagent-bridge import\n```", nil
	}

	// 3. Detect Implementation (Coding Agent)
	// The prompt will contain the ticket title/desc we generated above.
	if strings.Contains(prompt, "primes.py") || strings.Contains(prompt, "[PRIMES]") {
		// Return bash script to implement primes.py
		return `Here is the implementation for primes.py:

` + "```bash" + `
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

primes = [x for x in range(10000) if is_prime(x)]

with open('primes.json', 'w') as f:
    json.dump({'primes': primes}, f)
EOF

# Run it to generate output
python3 primes.py

# Commit and Push
git add primes.py primes.json
git commit -m "Add primes.py implementation" --author="Recac Agent <agent@recac.ai>" || echo "No changes to commit"
git push

# Mark feature as implemented
agent-bridge feature set req-primes-py-exists --status "Done" --passes true
agent-bridge feature set req-primes-json-exists-and-contain --status "Done" --passes true
agent-bridge feature set req-primes-json-contains-exactly-1 --status "Done" --passes true
` + "```" + `
`, nil
	}

	// Default fallback
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100))

	// Add dummy command to prevent NO-OP loop detection if generic
	if strings.Contains(prompt, "QA AGENT") {
		// QA Agent expects to run tests
		response += "\n```bash\necho 'Running QA Checks...'\nagent-bridge signal QA_PASSED true\n```"
	} else {
		response += "\n```bash\necho 'Mock Agent Processing...'\n```"
	}

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
