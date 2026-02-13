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

	lowerPrompt := strings.ToLower(prompt)

	// --- 1. Technical Program Manager (TPM) Logic ---
	// The CLI `recac jira generate-from-spec` sends a prompt to TPM to create tickets.
	// It expects a JSON array of TicketSpec.
	if strings.Contains(lowerPrompt, "technical program manager") || strings.Contains(lowerPrompt, "generate-from-spec") {
		// Return a valid JSON array for the Prime Number scenario.
		// We use [PRIMES] as the ID prefix as requested by the scenario spec.
		return `[
  {
    "id": "PRIMES",
    "title": "[PRIMES] Create Prime Number Script",
    "description": "Create a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to 'primes.json'. commit both files.",
    "type": "Task",
    "acceptance_criteria": [
      "primes.py exists",
      "primes.json exists and contains correct primes"
    ]
  }
]`, nil
	}

	// --- 2. Coding Agent Logic ---
	// The `recac-agent` (Coding role) implements the feature.
	// The prompt will contain the feature description or "primes".
	// Prioritized over Initializer Logic to avoid infinite loops if history contains output of initializer.
	// BUT, we must ensure we don't trigger this in Turn 1 (where "primes" is in the task description but features aren't initialized yet).
	// We do this by requiring "agent-bridge import" to be present in the history (which is the output of the Initializer).
	// We check for "agent-bridge import" because "feature_list.json" might appear in the instructions of Turn 1.
	isCodingTrigger := strings.Contains(lowerPrompt, "primes") || strings.Contains(lowerPrompt, "python")
	hasInitializationHistory := strings.Contains(lowerPrompt, "agent-bridge import")

	if isCodingTrigger && hasInitializationHistory {
		// Return a python script wrapped in bash to write it to a file, run it, and git commit it.
		// We also mark the feature as completed in the database.
		return "```bash\n" +
			`# Create the python script
cat << 'EOF' > primes.py
import json

def get_primes(n):
    primes = []
    for num in range(2, n):
        is_prime = True
        for i in range(2, int(num ** 0.5) + 1):
            if num % i == 0:
                is_prime = False
                break
        if is_prime:
            primes.append(num)
    return primes

primes = get_primes(10000)
with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
EOF

# Run the script to generate the json
python3 primes.py

# Git commit
git add primes.py primes.json
git commit -m "feat: add prime number script and output" || echo "Nothing to commit"

# Mark feature as completed
agent-bridge feature set primes-script --status completed --passes true

# Signal project completion (optional but helpful for single-agent flows)
# agent-bridge signal set --key PROJECT_SIGNED_OFF --value "true" --privileged
` + "\n```", nil
	}

	// --- 3. Initializer Agent Logic ---
	// The `recac-agent` (Initializer role) runs first to bootstrap the project.
	// It expects to output a bash script that pipes a JSON feature list to `agent-bridge import`.
	// The prompt usually contains "You are an Initializer Agent" or similar.
	// Moved below Coding Agent check (with history dependency) to handle Turn 1 correctly.
	if strings.Contains(lowerPrompt, "initializer agent") || strings.Contains(lowerPrompt, "feature_list.json") {
		// Return a bash script that creates the feature list and imports it.
		// Note: We include the feature ID "primes-script" which the Coding Agent will later reference.
		return "```bash\n" +
			`cat << 'EOF' > feature_list.json
{
  "features": [
    {
      "id": "primes-script",
      "name": "Create Prime Number Script",
      "description": "Implement primes.py and generate primes.json",
      "status": "todo",
      "file_paths": ["primes.py", "primes.json"]
    }
  ]
}
EOF

# Import the feature list using agent-bridge
cat feature_list.json | agent-bridge import
` + "\n```", nil
	}

	// --- 4. QA/Review Agent Logic ---
	// If the loop continues to a QA phase or if the agent is asked to review.
	if strings.Contains(lowerPrompt, "qa") || strings.Contains(lowerPrompt, "review") || strings.Contains(lowerPrompt, "verify") {
		return "LGTM. The code looks correct and meets the requirements.", nil
	}

	// Default fallback response
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
