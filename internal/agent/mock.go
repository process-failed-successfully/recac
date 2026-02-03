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

	// Normalize for case-insensitive matching
	promptLower := strings.ToLower(prompt)

	// Smart Mock Logic for Smoke Tests

	// 0. Initializer / Feature List Request (Priority 0)
	// This must come BEFORE Ticket Generation because the prompt for Initializer often contains
	// the ticket ID and "JSON format" which triggers the Ticket Generation rule.
	// Matches "initializer" (case-insensitive) OR "feature_list" (handles feature_list.json)
	if (strings.Contains(promptLower, "initializer") || strings.Contains(promptLower, "feature_list") || strings.Contains(promptLower, "feature list")) && strings.Contains(prompt, "ID:[PRIMES]") {
		return `I have identified the required features.

` + "```bash" + `
cat << 'EOF' > feature_list.json
[
  {
    "id": "PRIMES",
    "name": "Primes Script",
    "description": "Python script to calculate primes",
    "file": "primes.py"
  }
]
EOF
# Import into database as required by system
cat feature_list.json | agent-bridge import
echo "Feature list created and imported."
` + "```" + `
`, nil
	}


	// 1. Ticket Generation Request (Prime Python Scenario)
	// Must ensure this doesn't trigger on Initializer prompts
	if strings.Contains(prompt, "ID:[PRIMES]") && strings.Contains(prompt, "JSON format") &&
	   !strings.Contains(promptLower, "initializer") && !strings.Contains(promptLower, "feature_list") && !strings.Contains(promptLower, "feature list") {
		return `[
  {
    "title": "ID:[PRIMES] [GEN] Create Prime Number Script",
    "description": "Create a python script named 'primes.py' that calculates primes < 10000 and outputs to 'primes.json'. ID:[PRIMES]",
    "type": "Task",
    "children": []
  }
]`, nil
	}

	// 2. Implementation Request (Writing the file)
	// Matches prompt asking to implement "PRIMES" or "primes.py"
	// Must not be Initializer (which might also mention the filename in context)
	if (strings.Contains(prompt, "PRIMES") || strings.Contains(prompt, "primes.py")) &&
		!strings.Contains(promptLower, "initializer") {
		return `I will create the primes.py script and the json output as requested.

` + "```bash" + `
# Create the python script
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

primes_list = get_primes(10000)
with open("primes.json", "w") as f:
    json.dump({"primes": primes_list}, f)
EOF

# Run the script to generate the json
python3 primes.py

# Add and commit
git add primes.py primes.json
git commit -m "Add primes script and output" || echo "Nothing to commit"

# Signal completion
agent-bridge feature set PRIMES --status done --passes true
` + "```" + `
`, nil
	}

	// 3. QA Agent Request
	if strings.Contains(prompt, "YOUR ROLE - QA AGENT") {
		return `I have verified the project and it looks good.

` + "```bash" + `
echo "QA verification passed"
agent-bridge signal QA_PASSED true
` + "```" + `
`, nil
	}

	// 4. Project Manager Request
	if strings.Contains(prompt, "YOUR ROLE - PROJECT MANAGER") {
		return `I have reviewed the project and it meets all requirements.

` + "```bash" + `
echo "Project Manager sign-off approved"
agent-bridge signal PROJECT_SIGNED_OFF true
` + "```" + `
`, nil
	}

	// Default Mock Response
	// We include a no-op bash block to ensure the executor doesn't trip the "no commands" circuit breaker
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...\n\n```bash\n# no-op to prevent circuit breaker\necho 'mock agent alive'\n```",
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
