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

	// [QA] Role Detection
	if strings.Contains(prompt, "ROLE - QA AGENT") || strings.Contains(prompt, "verify the project") {
		return m.generateQAResponse(), nil
	}

	// [Manager] Role Detection
	if strings.Contains(prompt, "ROLE - PROJECT MANAGER") {
		return m.generateManagerResponse(), nil
	}

	// [INITIALIZER] Role Detection
	// Check for "ROLE - INITIALIZER AGENT" specifically to generate feature_list.json
	// This must be checked before [PRIMES] generic logic to prevent the coding script from being returned.
	if strings.Contains(prompt, "ROLE - INITIALIZER AGENT") {
		if strings.Contains(prompt, "[PRIMES]") || strings.Contains(prompt, "primes.py") {
			return m.generatePrimesFeatureListResponse(), nil
		}
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

		// Unified response: Check state and act accordingly
		// This handles both implementation and completion signaling in a single robust script
		// effectively preventing "nothing to commit" false positives from stopping implementation.
		return m.generatePrimesSmartResponse(), nil
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

func (m *MockAgent) generatePrimesFeatureListResponse() string {
	script := `
cat << 'EOF' | agent-bridge import
{
  "project_name": "Primes Project",
  "features": [
    {
      "id": "req-must-correctly-identify-prime-",
      "category": "functional",
      "priority": "MVP",
      "description": "Implement primes.py script that calculates primes correctly and outputs to primes.json",
      "status": "pending",
      "steps": [
        "Step 1: Run python3 primes.py",
        "Step 2: Check if primes.json exists",
        "Step 3: Verify content of primes.json"
      ],
      "passes": false,
      "dependencies": {
        "depends_on_ids": [],
        "exclusive_write_paths": ["primes.py", "primes.json"],
        "read_only_paths": []
      }
    }
  ]
}
EOF

# Initialize repo as requested
git init
echo "Primes Project" > README.md
git add README.md
git commit -m "Initial commit" || echo "No changes to commit"
`
	return fmt.Sprintf("I will initialize the project and create the feature list.\n\n```bash%s```\n", script)
}

func (m *MockAgent) generatePrimesSmartResponse() string {
	script := `
if [ -f "primes.py" ] && [ -f "primes.json" ]; then
    echo "Files exist. Checking if task is marked done..."
    # Mark as done using correct positional argument syntax for ID
    agent-bridge feature set "req-must-correctly-identify-prime-" --status done --passes true
else
    echo "Implementing primes.py..."
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
fi
`
	return fmt.Sprintf("I will check the state and implement primes.py if missing.\n\n```bash%s```\n", script)
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

func (m *MockAgent) generateQAResponse() string {
	script := `
echo "Running QA tests..."
# Simulate test run
echo "PASS"
agent-bridge signal QA_PASSED true
`
	return fmt.Sprintf("I will verify the project.\n\n```bash%s```\n", script)
}

func (m *MockAgent) generateManagerResponse() string {
	script := `
echo "Manager Review: Approved."
# Ensure feature is marked as passed to prevent premature sign-off detection
agent-bridge feature set req-must-correctly-identify-prime- --status done --passes true
agent-bridge signal PROJECT_SIGNED_OFF true --privileged
`
	return fmt.Sprintf("I approve the project.\n\n```bash%s```\n", script)
}

// truncateString truncates a string to a maximum length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
