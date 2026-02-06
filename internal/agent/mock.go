package agent

import (
	"context"
	"fmt"
	"strings"
)

// MockAgent is a smart mock agent for testing and smoke tests
// It returns specific bash scripts based on the prompt content to pass E2E scenarios
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

	// [PRIMES] Scenario Logic
	// Detect if we are being asked to implement the primes.py script
	if strings.Contains(prompt, "primes.py") || strings.Contains(prompt, "[PRIMES]") {
		// Differentiate between TPM (Planning) and Coding Agent (Implementation)
		// We broaden the check because templates might vary slightly or casing might differ.
		// "Epics" and "User Stories" are very specific to the TPM task in this project.
		if strings.Contains(prompt, "Technical Program Manager") ||
			strings.Contains(prompt, "tpm_agent") ||
			strings.Contains(prompt, "Epics") ||
			strings.Contains(prompt, "User Stories") {
			return m.generatePrimesJSONResponse(), nil
		}

		// Check if we have already implemented the script (git commit reported no changes)
		// This prevents infinite loops where the agent keeps trying to commit the same code
		if strings.Contains(prompt, "nothing to commit") {
			return m.generatePrimesCompletionResponse(), nil
		}

		return m.generatePrimesResponse(), nil
	}

	// [INITIALIZER] Logic
	// Detect if we are initializing the repo (Orchestrator might send "Initializing..." or similar,
	// but usually the agent is started with a goal.
	// If the prompt mentions "git init" or "setup", we might need a specific response.
	// For now, let's provide a generic helpful response that tries to prevent "NO-OP LOOP".

	return fmt.Sprintf("%s:\n\nI received your prompt (%d characters). I am ready to work.\n\n```bash\nls -la\n```\n",
		m.responsePrefix, len(prompt)), nil
}

// SendStream implements the Agent interface
func (m *MockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	resp, err := m.Send(ctx, prompt)
	if err == nil && onChunk != nil {
		onChunk(resp)
	}
	return resp, err
}

func (m *MockAgent) generatePrimesResponse() string {
	script := `
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n < 2: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = [p for p in range(10000) if is_prime(p)]
with open('primes.json', 'w') as f:
    json.dump({'primes': primes}, f)
EOF

python3 primes.py
git add primes.py primes.json
git commit -m "Implement primes.py"
git push origin HEAD
`
	return fmt.Sprintf("I will implement the primes.py script as requested.\n\n```bash%s```\n", script)
}

func (m *MockAgent) generatePrimesCompletionResponse() string {
	// We use agent-bridge to explicitly mark the feature as done.
	// This signals the runner to stop the loop.
	script := `
echo "Task completed. Updating status."
agent-bridge feature set --id "$RECAC_PROJECT_ID" --status done
`
	return fmt.Sprintf("It seems the work is already done.\n\n```bash%s```\n", script)
}

func (m *MockAgent) generatePrimesJSONResponse() string {
	return "```json\n" +
		"[\n" +
		"  {\n" +
		"    \"title\": \"ID:[PRIMES] Create Prime Number Script\",\n" +
		"    \"description\": \"Implement a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to a file named 'primes.json'. Repo: https://github.com/process-failed-successfully/recac-jira-e2e\",\n" +
		"    \"type\": \"Task\",\n" +
		"    \"children\": []\n" +
		"  }\n" +
		"]\n" +
		"```\n"
}

// truncateString truncates a string to a maximum length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
