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

	// Helper for case-insensitive matching
	promptLower := strings.ToLower(prompt)

	// Heuristic for "prime-python" scenario planning phase.
	// The planner prompt includes the AppSpec.
	// We check for the specific ID used in the spec.
	// Use case-insensitive check for ID and spec keywords
	// EXCEPTION: Initializer prompt also contains spec, so we must exclude it.
	if strings.Contains(prompt, "ID:[PRIMES]") &&
		(strings.Contains(promptLower, "appspec") || strings.Contains(promptLower, "specification")) &&
		!strings.Contains(prompt, "INITIALIZER") {
		// Return a JSON ARRAY of tickets, as expected by cmd/recac/jira.go
		return `[
    {
      "title": "ID:[PRIMES] Create Prime Number Script",
      "description": "Create a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to 'primes.json'.",
      "type": "Task"
    }
  ]`, nil
	}

	// Heuristic for "prime-python" scenario execution phase.
	// We want to match the task execution prompt but NOT the planning prompt.
	// The planning prompt also contains "primes.py" and "Create", so we must exclude it.
	// Fix: Coding agent prompts DO contain "AppSpec" as context, so we can't just check for its presence.
	// Instead, we check if we are EXPLICITLY asked to plan (e.g. "Create a plan", "Break down").
	isExplicitPlanningRequest := strings.Contains(promptLower, "create a plan") || strings.Contains(promptLower, "break down")

	// Check for implementation triggers:
	// 1. Task ID: [PRIMES] (often in prompt as "Task: [PRIMES]" or similar)
	// 2. File + Action: "primes.py" AND "create" (case insensitive)
	isImplementation := strings.Contains(prompt, "[PRIMES]") || (strings.Contains(promptLower, "primes.py") && strings.Contains(promptLower, "create"))

	if !isExplicitPlanningRequest && isImplementation {
		return `
I will implement the prime number script.

` + "```bash" + `
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n < 2: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = [p for p in range(10000) if is_prime(p)]

with open("primes.json", "w") as f:
    json.dump({"primes": primes}, f)
EOF

python3 primes.py

# Add to git
git config user.email "mock-agent@recac.io"
git config user.name "Mock Agent"
git add primes.py primes.json
git commit -m "Add primes.py and primes.json"

# Signal Completion (Smoke Test Support)
if command -v agent-bridge >/dev/null 2>&1; then
  # Dynamically find the feature ID and update it (handles environment injection)
  FEATURE_ID=$(agent-bridge feature list --json | jq -r '.features[0].id')
  if [ -n "$FEATURE_ID" ] && [ "$FEATURE_ID" != "null" ]; then
      agent-bridge feature update "$FEATURE_ID" --status done
  fi
  agent-bridge signal COMPLETED true
fi
` + "```" + `
`, nil
	}

	// Heuristic for QA Agent
	if strings.Contains(prompt, "QA AGENT") {
		return `I will verify the project.
` + "```bash" + `
agent-bridge signal QA_PASSED true
` + "```" + `
`, nil
	}

	// Heuristic for Project Manager
	if strings.Contains(prompt, "PROJECT MANAGER") {
		return `I will sign off.
` + "```bash" + `
agent-bridge signal PROJECT_SIGNED_OFF true
` + "```" + `
`, nil
	}

	// Heuristic for Initializer (create feature_list.json)
	// Trigger: Prompt mentions "feature_list.json" or "Initialize" AND we are not just reporting success.
	// EXCEPTION: If prompt mentions "feature_list.json already exists", we assume it's done and don't trigger again.
	// EXCEPTION: If prompt mentions "[PRIMES]", we prefer Implementation logic (handled above, but fallthrough might happen if logic above isn't triggered).
	// Actually, Implementation logic above has !isPlanning. If isPlanning is true, we might fall here.
	// But Planning is handled by the first block.

	shouldInitialize := (strings.Contains(prompt, "feature_list.json") || strings.Contains(prompt, "Initialize")) &&
		!strings.Contains(prompt, "feature_list.json already exists")

	if shouldInitialize {
		// Only run if we haven't already done it (avoid infinite loop)
		// We check if prompt implies current state has it? Hard to know.
		// We rely on the script to be idempotent/safe.
		return `
I will create the feature list.

` + "```bash" + `
if [ -f feature_list.json ]; then
  echo "feature_list.json already exists."
else
cat << 'EOF' > feature_list.json
{
    "project_name": "Prime Number Script",
    "features": [
        {
            "id": "1",
            "category": "core",
            "priority": "MVP",
            "description": "Calculate primes under 10000",
            "status": "todo",
            "passes": false,
            "steps": [],
            "dependencies": {
                "depends_on_ids": [],
                "exclusive_write_paths": [],
                "read_only_paths": []
            }
        }
    ]
}
EOF
fi

# Import it if agent-bridge is available
if command -v agent-bridge >/dev/null 2>&1; then
  cat feature_list.json | agent-bridge import || echo "Import skipped but continuing (mock mode)"
else
  echo "agent-bridge not found (mock mode)"
fi
` + "```" + `
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
