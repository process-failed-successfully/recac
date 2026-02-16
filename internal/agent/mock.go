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

	// Heuristics for E2E scenarios
	lowerPrompt := strings.ToLower(prompt)

	// Phase 1: TPM Agent (Ticket Generation)
	// Matches prompts containing "technical program manager" and the specific scenario markers
	if strings.Contains(lowerPrompt, "technical program manager") {
		if strings.Contains(prompt, "ID:[PRIMES]") || strings.Contains(lowerPrompt, "prime number script") {
			return `[
  {
    "title": "ID:[PRIMES] Create Prime Number Script",
    "description": "Implement a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to a file named 'primes.json'.Repo: https://github.com/process-failed-successfully/recac-jira-e2e",
    "type": "Task",
    "children": []
  }
]`, nil
		}
	}

	// Phase 2: Coding Agent (Task Execution)
	// Matches prompts containing "coding agent" and the specific scenario markers
	if strings.Contains(lowerPrompt, "coding agent") {
		if strings.Contains(prompt, "ID:[PRIMES]") || strings.Contains(lowerPrompt, "prime number script") || strings.Contains(lowerPrompt, "primes.py") {
			// Extract Task ID to update status
			taskID := "UNKNOWN"
			re := regexp.MustCompile(`Feature ID: ([A-Za-z0-9-]+)`)
			matches := re.FindStringSubmatch(prompt)
			if len(matches) > 1 {
				taskID = matches[1]
			}

			// Return bash commands to implement the solution and signal completion
			return fmt.Sprintf(`I will implement the prime number script as requested.

`+"```bash"+`
# Create the python script
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n < 2:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n %% i == 0:
            return False
    return True

primes = [x for x in range(10000) if is_prime(x)]

with open('primes.json', 'w') as f:
    json.dump({'primes': primes}, f)
EOF

# Run the script to generate output
python3 primes.py

# Commit changes
git add primes.py primes.json
git commit -m "Implement primes script"

# Signal completion
agent-bridge feature set %s --status done --passes true
agent-bridge signal COMPLETED true
`+"```"+`
`, taskID), nil
		}
	}

	// Phase 3: QA Agent
	// Matches prompts containing "qa agent"
	if strings.Contains(lowerPrompt, "qa agent") {
		return `I will verify the project.

`+"```bash"+`
# Run verification (assuming it passes for mock)
echo "Tests passed"
agent-bridge signal QA_PASSED true
`+"```"+`
`, nil
	}

	// Phase 4: Manager Review
	// Matches prompts containing "project manager"
	if strings.Contains(lowerPrompt, "project manager") {
		return `I approve the project.

`+"```bash"+`
agent-bridge signal PROJECT_SIGNED_OFF true
`+"```"+`
`, nil
	}

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
