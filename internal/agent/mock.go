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
		// Detect completion loop (nothing left to commit)
		// We check for "No changes to commit" because our script echoes this when git commit fails due to no changes.
		if strings.Contains(prompt, "nothing to commit") || strings.Contains(prompt, "No changes to commit") {
			return m.generatePrimesCompletionResponse(), nil
		}

		// Initializer (Needs Bash Plan + Init)
		if strings.Contains(prompt, "ROLE - INITIALIZER AGENT") {
			return m.generatePrimesInitializerResponse(), nil
		}

		// Technical Program Manager (Needs JSON Plan for Jira)
		if strings.Contains(prompt, "Technical Program Manager") ||
			strings.Contains(prompt, "tpm_agent") ||
			strings.Contains(prompt, "Epics") ||
			strings.Contains(prompt, "User Stories") {
			return m.generatePrimesJSONResponse(), nil
		}

		// Default: Coding Agent (Needs Implementation)
		return m.generatePrimesResponse(), nil
	}

	// [INITIALIZER] Generic Logic (Fallback if [PRIMES] tag is missing but Role is Initializer)
	if strings.Contains(prompt, "ROLE - INITIALIZER AGENT") {
		// Just ls -la to show activity if we don't know the scenario
		return fmt.Sprintf("%s:\n\nI received your prompt. Ready to init.\n\n```bash\nls -la\n```\n", m.responsePrefix), nil
	}

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

func (m *MockAgent) generatePrimesInitializerResponse() string {
	script := `
cat <<EOF | agent-bridge import
{
  "project_name": "$RECAC_PROJECT_ID",
  "features": [
    {
      "id": "req-must-correctly-identify-prime-",
      "category": "core",
      "priority": "1",
      "description": "Script calculates primes correctly",
      "status": "todo",
      "dependencies": {
        "depends_on_ids": []
      }
    }
  ]
}
EOF

cat << 'EOF' > init.sh
#!/bin/bash
echo "Initializing environment..."
# Install python3 if missing (it's usually there)
command -v python3 >/dev/null 2>&1 || apt-get update && apt-get install -y python3
EOF
chmod +x init.sh
./init.sh

git init
git add .
git commit -m "Initial commit" || echo "Nothing to commit"
`
	return fmt.Sprintf("I have analyzed the requirements. Here is the feature list and initialization script.\n\n```bash%s```\n", script)
}

func (m *MockAgent) generatePrimesResponse() string {
	// Coding Agent implementation
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
git commit -m "Implement primes.py" || echo "No changes to commit"
git push origin HEAD
`
	return fmt.Sprintf("I will implement the primes.py script as requested.\n\n```bash%s```\n", script)
}

func (m *MockAgent) generatePrimesCompletionResponse() string {
	return "Task appears complete. Marking as done.\n\n```bash\nagent-bridge feature set --status done --passes true\n```\n"
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
