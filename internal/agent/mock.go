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
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// 1. Initializer Agent - Sets up the repo
	// Check for "INITIALIZER" (uppercase) as used in prompts/templates/initializer.md
	// MOVED TO TOP to prevent TPM/Generic heuristics from catching "Application Specification" in the prompt
	// CRITICAL: We must be specific to the ROLE header to avoid matching "git init" in the history of other agents.
	if strings.Contains(prompt, "ROLE - INITIALIZER AGENT") {
		// Extract Repo URL if present
		repoURL := ""
		lines := strings.Split(prompt, "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "Repo:") {
				repoURL = strings.TrimSpace(strings.TrimPrefix(trimmed, "Repo:"))
				break
			}
		}

		// Prepare git setup command
		// If GITHUB_API_KEY is set and we have a repo URL, try to clone.
		// Otherwise fallback to git init.
		// NOTE: We use sed for URL injection because bash parameter expansion ${VAR/../../}
		// is not fully portable to sh (often used in minimal containers).
		gitSetup := `
set -e # Fail fast
# Ensure clean slate
rm -rf .git

if [ -n "$GITHUB_API_KEY" ] && [ -n "` + repoURL + `" ]; then
  # Inject token into URL
  REPO_URL="` + repoURL + `"
  # Replace https:// with https://x-access-token:KEY@
  AUTH_URL=$(echo "$REPO_URL" | sed "s|https://|https://x-access-token:${GITHUB_API_KEY}@|")
  echo "Cloning from ${REPO_URL}..."
  git init
  git remote add origin "$AUTH_URL"
  git fetch origin
  git checkout -f master || git checkout -f main || echo "Failed to checkout default branch"
else
  echo "Initializing local repo (no token or url found)..."
  git init
fi

echo "Current Directory: $(pwd)"
ls -la
`

		// Detect Primes scenario
		if strings.Contains(strings.ToLower(prompt), "prime") {
			return `
I will initialize the repository and create the feature list for the prime number script.

` + "```bash" + gitSetup + `
git config user.email "you@example.com"
git config user.name "Your Name"

# Create feature list via agent-bridge import
cat << 'EOF' | agent-bridge import
{
  "project_name": "Prime Number Generator",
  "features": [
    {
      "id": "PRIMES",
      "category": "functional",
      "priority": "MVP",
      "description": "Implement a python script 'primes.py' that calculates primes < 10000 and outputs to 'primes.json'.",
      "status": "pending",
      "passes": false,
      "steps": [
        "Create primes.py",
        "Run python3 primes.py",
        "Verify primes.json exists"
      ],
      "dependencies": {
        "exclusive_write_paths": ["primes.py", "primes.json"],
        "read_only_paths": []
      }
    }
  ]
}
EOF
` + "```" + `
`, nil
		}

		return `
I will initialize the repository and import the plan.

` + "```bash" + gitSetup + `
git config user.email "you@example.com"
git config user.name "Your Name"
agent-bridge import --file /app/ticket_plan.json
` + "```" + `
`, nil
	}

	// 2. TPM Agent - Generates the plan
	// Removed "Application Specification" check as it is too broad and appears in Initializer prompt
	// Use case-insensitive check or check for uppercase role to be robust
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "TECHNICAL PROGRAM MANAGER") {
		return `
[
  {
    "id": "PRIMES",
    "type": "Task",
    "title": "ID:[PRIMES] Implement prime number script",
    "description": "Implement a python script 'primes.py' that calculates primes < 10000 and outputs to 'primes.json'."
  }
]
`, nil
	}

	// 3. Coding Agent - Implements the feature
	// We detect the [PRIMES] ID or the file request
	// Prioritize this before generic role checks if specific task ID is present
	if strings.Contains(prompt, "[PRIMES]") || strings.Contains(prompt, "primes.py") {
		return `
I will implement the prime number script as requested.

` + "```bash" + `
set -e
# Ensure git repo exists and is synced (fallback recovery)
if [ ! -d .git ]; then
  echo "Git repo missing, re-initializing..."
  git init
  git config user.email "you@example.com"
  git config user.name "Your Name"
else
  # Ensure we are on a valid branch if possible
  git fetch origin || echo "Fetch failed, ignoring"
fi

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

# Create or reset a branch for the feature (force if exists)
git checkout -B agent/PRIMES-mock

python3 primes.py
git add primes.py primes.json
git commit -m "Add primes.py and primes.json" || echo "Nothing to commit"
git push --force origin agent/PRIMES-mock || echo "Push failed, continuing local only"
agent-bridge feature set PRIMES --status done --passes true
` + "```" + `
`, nil
	}

	// 4. QA Agent - Verifies the project
	// Check for "QA AGENT" role header or "verify the project" instruction
	if strings.Contains(prompt, "ROLE - QA AGENT") || (strings.Contains(prompt, "verify") && strings.Contains(prompt, "project")) {
		return `
I will run the tests and verify the project status.

` + "```bash" + `
# Run verification (mock test)
if [ -f primes.json ]; then
  echo "primes.json exists, verifying content..."
  # Simple check if file is valid JSON and has content
  if grep -q "primes" primes.json; then
    echo "Verification Passed"
    agent-bridge signal QA_PASSED true
  else
    echo "Verification Failed: Invalid content"
    agent-bridge signal QA_PASSED false
  fi
else
  echo "Verification Failed: primes.json missing"
  agent-bridge signal QA_PASSED false
fi
` + "```" + `
`, nil
	}

	// 5. Project Manager - Signs off
	if strings.Contains(prompt, "ROLE - PROJECT MANAGER") {
		return `
Project Approved.

` + "```bash" + `
agent-bridge signal PROJECT_SIGNED_OFF true --privileged
` + "```" + `
`, nil
	}

	// 6. Default / Fallback
	// Return a mock response that shows the agent received the prompt
	fmt.Printf("[MockAgent] Hit Fallback! Prompt length: %d\nFull Prompt:\n%s\n", len(prompt), prompt)
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
