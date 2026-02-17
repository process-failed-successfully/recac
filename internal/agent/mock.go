package agent

import (
	"context"
	"fmt"
	"regexp"
)

var (
	// Regex patterns for detecting agent roles and tasks
	tpmRegex             = regexp.MustCompile(`(?i)Technical Program Manager`)
	architectRegex       = regexp.MustCompile(`(?i)(Lead Software Architect|break down)`)
	primesRegex          = regexp.MustCompile(`(?i)(ID:\[PRIMES\]|Implement prime calculation logic)`)
	qaRegex              = regexp.MustCompile(`(?i)QA AGENT`)
	managerRegex         = regexp.MustCompile(`(?i)Manager Agent`)
	pendingStatusRegex   = regexp.MustCompile(`"status":\s*"pending"`)
)

// MockAgent is a smarter mock agent for E2E testing
type MockAgent struct {
	responsePrefix string
	forcedResponse string
}

func NewMockAgent() *MockAgent {
	return &MockAgent{
		responsePrefix: "Mock agent response",
	}
}

func (m *MockAgent) SetResponse(response string) {
	m.forcedResponse = response
}

func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// 1. Check for Execution Phase (Task Completion)
	// Must check this before planning roles to avoid misinterpretation
	if pendingStatusRegex.MatchString(prompt) {
		return m.generateTaskCompletionScript(), nil
	}

	// 2. Check for Technical Program Manager (Planning Phase)
	if tpmRegex.MatchString(prompt) {
		return m.generateTPMResponse(), nil
	}

	// 3. Check for Architect Role
	if architectRegex.MatchString(prompt) {
		return m.generateArchitectResponse(), nil
	}

	// 4. Check for Primes Task (Special Scenario)
	if primesRegex.MatchString(prompt) {
		return m.generatePrimesScript(), nil
	}

	// 5. Check for QA Agent
	if qaRegex.MatchString(prompt) {
		return m.generateQAResponse(), nil
	}

	// 6. Check for Manager Agent
	if managerRegex.MatchString(prompt) {
		return m.generateManagerResponse(), nil
	}

	// Default response
	return fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response.",
		m.responsePrefix, len(prompt)), nil
}

func (m *MockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	resp, err := m.Send(ctx, prompt)
	if err == nil && onChunk != nil {
		onChunk(resp)
	}
	return resp, err
}

func (m *MockAgent) generateTPMResponse() string {
	// Returns a list of ticketNode objects
	return `[
  {
    "title": "ID:[INIT] Setup Project",
    "description": "Initialize the project repository and basic structure.",
    "type": "Epic",
    "children": [
      {
        "title": "Create basic folder structure",
        "description": "Create internal, cmd, pkg folders.",
        "type": "Story"
      }
    ]
  },
  {
    "title": "ID:[PRIMES] Implement Prime Logic",
    "description": "Create the primes.py file with prime number calculation logic.",
    "type": "Epic",
    "blocked_by": ["ID:[INIT] Setup Project"],
    "children": [
       {
          "title": "Write is_prime function",
          "description": "Implement the is_prime helper.",
          "type": "Story"
       },
       {
          "title": "Generate primes list",
          "description": "Generate list of primes up to 100.",
          "type": "Story"
       }
    ]
  }
]`
}

func (m *MockAgent) generateArchitectResponse() string {
	return `#!/bin/bash
echo '{"features": [{"id": "req-1", "description": "Prime calculation", "status": "pending"}]}' > feature_list.json
echo "Architectural breakdown complete."
`
}

func (m *MockAgent) generatePrimesScript() string {
	return `#!/bin/bash
cat <<EOF > primes.py
def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

import json
primes = [i for i in range(100) if is_prime(i)]
with open('primes.json', 'w') as f:
    json.dump(primes, f)
EOF
python3 primes.py
git config user.email "test@example.com" || true
git config user.name "Test User" || true
git add primes.py primes.json
git commit -m "Add prime calculation" || echo "Nothing to commit"
echo "Success: Mock command executed"
`
}

func (m *MockAgent) generateQAResponse() string {
	return `#!/bin/bash
agent-bridge signal QA_PASSED true
echo "QA Passed"
`
}

func (m *MockAgent) generateManagerResponse() string {
	return `#!/bin/bash
agent-bridge signal PROJECT_SIGNED_OFF true
echo "Project Signed Off"
`
}

func (m *MockAgent) generateTaskCompletionScript() string {
	return `#!/bin/bash
# Update feature list status to done
if [ -f feature_list.json ]; then
    sed -i 's/"status": "pending"/"status": "done"/g' feature_list.json
fi
echo "Task completed"
`
}
