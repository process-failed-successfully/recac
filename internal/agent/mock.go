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
// It returns a mock response that acknowledges the prompt or acts on heuristics for E2E tests
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// Heuristics for E2E Scenarios

	// 1. Initializer Agent (DB Import)
	if strings.Contains(prompt, "INITIALIZER AGENT") || (strings.Contains(prompt, "git init") && strings.Contains(prompt, "agent-bridge import")) {
		project := os.Getenv("RECAC_PROJECT_ID")
		if project == "" {
			project = "mock-project" // Match the default in workflow.go for tests
		}
		// Return a command to import features
		return fmt.Sprintf("```bash\ncat <<EOF | agent-bridge import\n{\n  \"project_name\": \"%s\",\n  \"features\": [\n    {\n      \"id\": \"req-implement-prime-calculation-lo\",\n      \"category\": \"core\",\n      \"priority\": \"high\",\n      \"description\": \"Implement a python script that calculates prime numbers up to 10000\",\n      \"status\": \"pending\",\n      \"dependencies\": {\"depends_on_ids\": []}\n    }\n  ]\n}\nEOF\n```", project), nil
	}

	// 2. TPM (Ticket Generation) - Prioritize this if prompt asks for ticket generation or JSON
	// Distinguish from Developer tasks
	if (strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "ticketNode")) && !strings.Contains(prompt, "ROLE - CODING AGENT") {
		return `
[
  {
    "id": "PRIMES",
    "title": "Implement Prime Number Calculation",
    "description": "Create a python script primes.py that calculates primes up to 10000.",
    "type": "Task",
    "status": "In Progress",
    "dependencies": [],
    "priority": "High"
  }
]
`, nil
	}

	// 3. Developer (Prime Python)
	if strings.Contains(prompt, "primes.py") || strings.Contains(prompt, "prime numbers") || strings.Contains(prompt, "req-implement-prime-calculation-lo") {
		// Detect if already done (via history in prompt)
		if strings.Contains(prompt, "status=done") || strings.Contains(prompt, "nothing to commit") {
			return "```bash\nagent-bridge signal QA_PASSED true\n```", nil
		}

		return `
I will implement the prime number calculation script.

` + "```bash" + `
cat <<EOF > primes.py
import json

def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

primes = [i for i in range(10001) if is_prime(i)]
with open("primes.json", "w") as f:
    json.dump({"primes": primes}, f)
EOF

# Run the script to generate the file
python3 primes.py

# Mark feature as done
# Use explicit ID from the initializer
agent-bridge feature set req-implement-prime-calculation-lo --status done || echo "Feature set failed, ignoring"

# Ensure clean exit for smoke tests
git add primes.py primes.json || true
git commit -m "Implement primes" || echo "No changes to commit"
` + "```" + `
`, nil
	}

	// 4. QA / Review
	if strings.Contains(prompt, "QA AGENT") {
		return "```bash\nagent-bridge signal set QA_PASSED true\n```", nil
	}

	if strings.Contains(prompt, "PROJECT MANAGER") && strings.Contains(prompt, "sign off") {
		return "```bash\nagent-bridge signal set PROJECT_SIGNED_OFF true\n```", nil
	}

	// Fallback
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
