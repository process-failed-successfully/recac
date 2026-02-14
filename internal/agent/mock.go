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

	// Heuristic: Check if this is a Planning/TPM prompt
	// This prompt is typically used by `recac jira generate-from-spec`
	if strings.Contains(prompt, "Technical Program Manager (TPM)") || strings.Contains(prompt, "agile software development") {
		// Return a JSON response that the CLI expects for planning
		// This simulates a breakdown of tasks into tickets
		return `[
  {
    "title": "ID:[PRIMES] Implement Prime Number Generator",
    "description": "Create a Python script that calculates prime numbers efficiently.\nRepo: https://github.com/process-failed-successfully/recac-jira-e2e",
    "type": "Story",
    "acceptance_criteria": [
      "Script runs without errors",
      "Calculates first 100 primes correctly",
      "Outputs results to primes.json"
    ],
    "children": []
  }
]`, nil
	}

	// Heuristic: Check if this is the "Coding Agent" working on the PRIMES task
	// The prompt typically contains the ticket ID or file name
	if strings.Contains(prompt, "ID:[PRIMES]") || strings.Contains(prompt, "primes.py") || strings.Contains(prompt, "1229") {
		// Return a response that simulates completing the task
		// We use agent-bridge commands to perform the work
		scriptContent := `
import json

def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = []
num = 2
while len(primes) < 100:
    if is_prime(num):
        primes.append(num)
    num += 1

with open("primes.json", "w") as f:
    json.dump(primes, f)

print(f"Generated {len(primes)} primes")
`
		// We need to escape newlines for the echo command or use a here-doc style if the agent supports it.
		// For simplicity, let's just write a simple one-liner or use printf.
		// Actually, `agent-bridge` doesn't execute the code itself, the agent typically outputs a shell block.

		response := "I will implement the prime number generator script.\n\n"
		response += "```bash\n"
		response += "cat <<EOF > primes.py" + scriptContent + "EOF\n"
		response += "python3 primes.py\n" // Run it to generate output
		response += "git add primes.py primes.json\n"
		response += "git commit -m 'Add prime number generator'\n"

		// Import requirements to satisfy the tool
		response += "agent-bridge import --source jiraticket\n"

		// Mark requirements as completed
		response += "agent-bridge feature set req-script-runs-without-errors completed\n"
		response += "agent-bridge feature set req-calculates-first-100-primes-co completed\n"
		response += "agent-bridge feature set req-outputs-results-to-primes-json completed\n"

		// Signal completion
		response += "agent-bridge signal PROJECT_SIGNED_OFF true --privileged\n"
		response += "```\n"

		return response, nil
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
