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

	// 0. Check for Initializer Agent (Session Bootstrap)
	// Matches against the Initializer Agent prompt structure (internal/agent/prompts/templates/initializer.md)
	// Case-insensitive check to be robust against template changes
	if strings.Contains(strings.ToUpper(prompt), "INITIALIZER") {
		return `I will initialize the project features.

` + "```bash" + `
cat << 'EOF' | agent-bridge import
{
  "project_name": "primes",
  "features": [
    {
      "id": "primes-impl",
      "description": "Create primes.py to calculate all prime numbers less than 10,000",
      "priority": "MVP",
      "status": "pending",
      "steps": [
        "Check if primes.py exists",
        "Run primes.py",
        "Check if primes.json exists"
      ],
      "passes": false,
      "dependencies": {
        "exclusive_write_paths": ["primes.py", "primes.json"]
      }
    }
  ]
}
EOF
` + "```" + `
`, nil
	}

	// 1. Check for Prime Python Scenario - Ticket Generation
	// Matches against the TPM Agent prompt structure (internal/agent/prompts/templates/tpm_agent.md)
	if strings.Contains(prompt, "Technical Program Manager") && strings.Contains(prompt, "ID:[PRIMES]") {
		return `[
  {
    "title": "ID:[PRIMES] Create Prime Number Script",
    "description": "Create a python script named 'primes.py'. It MUST be python.\\nIt must calculate all prime numbers less than 10,000 and output to a file named 'primes.json'.\\nIMPORTANT: You MUST use a bash block to create the file. Do not output raw python code.\\nCommit 'primes.py' and 'primes.json' IMMEDIATELY.\\nThe JSON format must have a single key 'primes' containing the list of integers.\\nExample: {\\\"primes\\\": [2, 3, 5, ...]}.\\nIMPORTANT: Ensure the FINAL primes.json committed to the repository contains ALL primes less than 10,000 (Exactly 1229 primes).\\nRepo: https://github.com/process-failed-successfully/recac-jira-e2e",
    "type": "Task"
  }
]`, nil
	}

	// 2. QA Agent Check
	if strings.Contains(prompt, "YOUR ROLE - QA AGENT") {
		return "QA Checks Passed.\n```bash\nagent-bridge signal QA_PASSED true\n```", nil
	}

	// 3. Project Manager Check
	if strings.Contains(prompt, "YOUR ROLE - PROJECT MANAGER") {
		return "Project Signed Off.\n```bash\nagent-bridge signal PROJECT_SIGNED_OFF true\n```", nil
	}

	// 4. Completion Check (Coding Agent Loop Break)
	// If the agent sees "nothing to commit", it means the implementation is done and committed.
	// We should mark the feature as done to stop the loop.
	if strings.Contains(prompt, "nothing to commit") || strings.Contains(prompt, "working tree clean") {
		return `Task appears complete. Updating status.

` + "```bash" + `
agent-bridge feature set primes-impl --status done --passes true || echo "Feature updated"
agent-bridge feature set req-primes --status done --passes true || echo "Feature updated"
` + "```" + `
`, nil
	}

	// 5. Check for Prime Python Scenario - Implementation
	// Looking for the ticket description content or keywords
	// We use a broader check (OR condition) to be robust against description formatting changes
	if strings.Contains(prompt, "primes.py") || strings.Contains(prompt, "primes-impl") || strings.Contains(prompt, "Prime Number Script") {
		return `I will implement the prime number script as requested.

` + "```bash" + `
# Configure git
git config user.email "bot@recac.com"
git config user.name "Recac Bot"

# Create the python script
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n < 2: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = [p for p in range(10000) if is_prime(p)]
with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
EOF

# Run it to generate the json
python3 primes.py

# Commit files
git add primes.py primes.json
git commit -m "Add primes.py and primes.json" || echo "Nothing to commit"
git push
` + "```" + `

Implementation complete.
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
