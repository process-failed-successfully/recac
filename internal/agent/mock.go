package agent

import (
	"context"
	"fmt"
	"strings"
)

// MockAgent is a smart mock agent for testing and mock mode
// It returns predefined responses based on triggers in the prompt
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

	// Break loop if we see "nothing to commit" in the prompt, indicating idempotency
	if strings.Contains(prompt, "nothing to commit") || strings.Contains(prompt, "working tree clean") {
		return `It seems the work is already done and committed.
` + "```bash" + `
# no-op
if command -v agent-bridge >/dev/null 2>&1; then
    agent-bridge update --status done
fi
` + "```", nil
	}

	// 1. Initializer: feature_list.json
	// We check for "Initialize" or "feature_list.json" but exclude "generate-from-spec" context if possible,
	// though Initializer usually runs first.
	if strings.Contains(prompt, "Initialize") || strings.Contains(prompt, "feature_list.json") {
		// Create feature_list.json
		// We use echo to avoid Heredoc complexity if needed, but cat << 'EOF' is standard.
		// Memory says: "generates a response using echo ... to create the file"
		// But let's use cat << 'EOF' as it is cleaner for multiline JSON.
		// Adding set -x for debugging.
		return `I will initialize the project.
` + "```bash" + `
set -x
# Create feature_list.json
cat << 'EOF' > feature_list.json
{
  "project_name": "Prime Calculation",
  "features": [
    {
      "id": "req-primes",
      "description": "Calculate primes < 10000",
      "status": "todo"
    }
  ]
}
EOF

if command -v agent-bridge >/dev/null 2>&1; then
    agent-bridge update --status in_progress
fi
` + "```", nil
	}

	// 2. Implementation: Prime Python
	// Triggers: req-primes, [PRIMES], primes.py
	if strings.Contains(prompt, "req-primes") || strings.Contains(prompt, "[PRIMES]") || strings.Contains(prompt, "primes.py") {
		// Python script to calculate primes
		pythonScript := `
import json

primes = []
for num in range(2, 10000):
    is_prime = True
    for i in range(2, int(num ** 0.5) + 1):
        if num % i == 0:
            is_prime = False
            break
    if is_prime:
        primes.append(num)

with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
print(f"Calculated {len(primes)} primes.")
`
		return `I will implement the prime number script.
` + "```bash" + `
set -x
# Configure git
git config user.email "agent@recac.com"
git config user.name "Recac Agent"

# Create script
cat << 'EOF' > primes.py` + pythonScript + `EOF

# Run script
python3 primes.py || python primes.py

# Commit
git add primes.py primes.json
git commit -m "Add primes.py and primes.json" || echo "Nothing to commit"

# Update feature status
if command -v agent-bridge >/dev/null 2>&1; then
    agent-bridge update --id req-primes-py-exists --status done
    agent-bridge update --id req-primes-json-contains-correct-p --status done
    agent-bridge update --status done
fi
` + "```", nil
	}

	// 3. QA Agent
	if strings.Contains(prompt, "QA AGENT") {
		return `I have verified the changes.
` + "```bash" + `
if command -v agent-bridge >/dev/null 2>&1; then
    agent-bridge signal QA_PASSED true
fi
` + "```", nil
	}

	// 4. Project Manager
	if strings.Contains(prompt, "PROJECT MANAGER") {
		return `Approved.
` + "```bash" + `
if command -v agent-bridge >/dev/null 2>&1; then
    agent-bridge signal PROJECT_SIGNED_OFF true
fi
` + "```", nil
	}

    // 5. Generate Plan (Technical Program Manager)
    // Check for "generate-from-spec" or "technical program manager"
    lowerPrompt := strings.ToLower(prompt)
    if (strings.Contains(lowerPrompt, "generate-from-spec") || strings.Contains(lowerPrompt, "technical program manager")) &&
       !strings.Contains(prompt, "Initializer") && !strings.Contains(prompt, "feature list") {
           return `Here is the plan.
` + "```json" + `
[
  {
    "id": "req-primes",
    "title": "Primes Script",
    "description": "Calculate primes less than 10000",
    "type": "Task"
  }
]
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

// truncateString truncates a string to a maximum length and escapes backticks
func truncateString(s string, maxLen int) string {
	if len(s) > maxLen {
		s = s[:maxLen]
	}
	return strings.ReplaceAll(s, "`", "'")
}
