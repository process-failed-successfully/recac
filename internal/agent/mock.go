package agent

import (
	"context"
	"fmt"
	"regexp"
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

	// [PRIMES] Scenario Logic - Differentiate by Role

	// 1. TPM Agent (Ticket Generation)
	// Check for keywords specific to the TPM prompt
	if (strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "Ticket Generation") || strings.Contains(prompt, "decompose it into a series")) && strings.Contains(prompt, "[PRIMES]") {
		// Extract Repo URL from prompt
		repoURL := "https://github.com/process-failed-successfully/recac-jira-e2e" // Default fallback for CI
		re := regexp.MustCompile(`Repo:\s*(https?://\S+)`)
		if matches := re.FindStringSubmatch(prompt); len(matches) > 1 {
			repoURL = matches[1]
		}

		return fmt.Sprintf(`
[
  {
    "title": "ID:[PRIMES] Prime Number Script",
    "description": "Implement a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to a file named 'primes.json'. Repo: %s",
    "type": "Task",
    "children": []
  }
]
`, repoURL), nil
	}

	// 2. Coding Agent (Implementation)
	// Check for keywords specific to the Coding Agent prompt, or generic [PRIMES] request if not TPM
	// "CODING AGENT" is in the coding_agent.md template header
	if (strings.Contains(prompt, "CODING AGENT") || strings.Contains(prompt, "primes.py")) && strings.Contains(prompt, "[PRIMES]") {
		return `
I will implement the prime number script as requested.

Here is the implementation:

` + "```bash" + `
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n < 2: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = [x for x in range(10000) if is_prime(x)]
with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
EOF

# Run the script to generate the JSON file
python3 primes.py

# Commit the changes
git add primes.py primes.json
git commit -m "Implement primes calculation script"

# Signal completion
agent-bridge feature set PRIMES --status done --passes true
agent-bridge signal COMPLETED true
` + "```" + `
`, nil
	}

	// 3. QA Agent
	// Check for "QA AGENT" or "YOUR ROLE - QA AGENT"
	if strings.Contains(prompt, "QA AGENT") || strings.Contains(prompt, "YOUR ROLE - QA AGENT") {
		return `
I will run the tests and verify the project.

` + "```bash" + `
# Dummy test for mock scenario
echo "Running tests..."
# Verify that primes.json exists (basic check for the PRIMES scenario)
if [ -f primes.json ]; then
    echo "PASS: primes.json found"
else
    echo "WARNING: primes.json not found, but assuming PASS for mock"
fi

# Signal QA Passed
agent-bridge signal QA_PASSED true
` + "```" + `
`, nil
	}

	// Default Response
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
