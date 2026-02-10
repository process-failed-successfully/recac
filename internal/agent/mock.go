package agent

import (
	"context"
	"fmt"
	"strings"
)

// MockAgent is a smart mock agent for testing and mock mode
type MockAgent struct {
	responsePrefix string
	forcedResponse string
	iterationCount int
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
	m.iterationCount++

	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// 1. TPM Agent (Ticket Generation)
	// Heuristic: "You are an expert Technical Program Manager (TPM)"
	if strings.Contains(prompt, "Technical Program Manager (TPM)") {
		// Detect scenario from spec content if possible, or just default to prime-python if "primes" is mentioned
		if strings.Contains(strings.ToLower(prompt), "prime") {
			return `
Here is the plan:
` + "```json" + `
[
  {
    "title": "ID:[PRIMES] Create Prime Number Script",
    "description": "Create a python script named 'primes.py'. It MUST be python.\nIt must calculate all prime numbers less than 10,000 and output to a file named 'primes.json'.\nRepo: https://github.com/process-failed-successfully/recac-jira-e2e",
    "type": "Task",
    "children": []
  }
]
` + "```", nil
		}
	}

	// 2. Initializer Agent
	// Heuristic: "## YOUR ROLE - INITIALIZER AGENT" (Assuming template header)
	if strings.Contains(prompt, "INITIALIZER AGENT") || strings.Contains(prompt, "Initialize the repository") {
		return `
I will initialize the repository and create the feature tracking file.

` + "```bash" + `
# Initialize repository if needed
git init
git config user.email "agent@recac.com"
git config user.name "Recac Agent"

# Import features
cat << 'EOF' | agent-bridge import
{
  "project_name": "Prime Script",
  "features": [
    {
      "id": "PRIMES",
      "description": "Create Prime Number Script",
      "category": "functional",
      "priority": "MVP",
      "status": "pending",
      "steps": ["Run primes.py", "Check primes.json"],
      "dependencies": {
        "exclusive_write_paths": ["primes.py", "primes.json"]
      }
    }
  ]
}
EOF

# Add and commit
git add feature_list.json
git commit -m "Initialize project structure"
` + "```", nil
	}

	// 3. Coding Agent
	// Heuristic: "## YOUR ROLE - CODING AGENT"
	if strings.Contains(prompt, "CODING AGENT") {
		// Break infinite loop if called too many times
		if m.iterationCount > 10 {
			return `
I have completed the work.

` + "```bash" + `
agent-bridge feature set PRIMES --status done --passes true
` + "```", nil
		}

		if strings.Contains(prompt, "PRIMES") {
			return `
I will implement the prime number script.

` + "```bash" + `
# Create primes.py
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

primes = [x for x in range(10000) if is_prime(x)]

with open("primes.json", "w") as f:
    json.dump({"primes": primes}, f)
EOF

# Run it to generate the JSON
python3 primes.py

# Commit
git add primes.py primes.json
git commit -m "Implement primes script"

# Signal completion
agent-bridge feature set PRIMES --status done --passes true
` + "```", nil
		}
	}

	// 4. QA Agent
	// Heuristic: "## YOUR ROLE - QA AGENT"
	if strings.Contains(prompt, "QA AGENT") {
		return `
I will verify the implementation.

` + "```bash" + `
# Verify file existence
if [ -f "primes.json" ]; then
  echo "primes.json exists"
else
  echo "primes.json missing"
  exit 1
fi

# Basic content check
grep -q "primes" primes.json && echo "JSON key found"

# Signal success
agent-bridge signal --privileged QA_PASSED true
` + "```" + `

QA Status: PASSED
`, nil
	}

	// 5. Project Manager (Sign off)
	// Heuristic: "## YOUR ROLE - PROJECT MANAGER" or "manager_review"
	if strings.Contains(prompt, "PROJECT MANAGER") || strings.Contains(prompt, "Review QA Report") {
		return `
The work looks good. I am signing off.

` + "```bash" + `
agent-bridge signal COMPLETED true
` + "```", nil
	}

	// Default response
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
