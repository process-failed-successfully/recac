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

	// Heuristic: Initializer (Feature List)
	// Usually prompted with "create a file named feature_list.json" or similar
	if (strings.Contains(prompt, "feature_list.json") || strings.Contains(prompt, "Create a list of features")) && !strings.Contains(prompt, "CODING AGENT") {
		return `I will create the feature list.
` + "```bash" + `
cat <<EOF > feature_list.json
{
  "features": [
    {
      "id": "req-must-correctly-identify-prime-",
      "description": "ID:[PRIMES] Implement a python script named 'primes.py' to calculate prime numbers",
      "status": "todo",
      "passes": false
    }
  ]
}
EOF
` + "```", nil
	}

	// Heuristic: TPM (Ticket Generation)
	// Prompt contains "Type: Task" and "CRITICAL INSTRUCTION: You MUST create exactly ONE ticket"
	if strings.Contains(prompt, "Type: Task") && strings.Contains(prompt, "CRITICAL INSTRUCTION: You MUST create exactly ONE ticket") {
		// Return JSON ticket list for [PRIMES]
		return "```json\n" + `[
  {
    "title": "ID:[PRIMES] Create Prime Number Script",
    "description": "Create a python script named 'primes.py'. It MUST be python.\nIt must calculate all prime numbers less than 10,000 and output to a file named 'primes.json'.\nIMPORTANT: You MUST use a bash block to create the file.",
    "type": "Task",
    "children": []
  }
]` + "\n```", nil
	}

	// Heuristic: Coding Agent (Prime Script)
	// Prompt asks to implement 'primes.py' or '[PRIMES]' or the specific task description
	if strings.Contains(prompt, "Create a python script named 'primes.py'") || strings.Contains(prompt, "[PRIMES]") || strings.Contains(prompt, "calculate prime numbers") {
		// Return bash script to create primes.py AND primes.json
		// We use feature set to mark as done if possible
		return `I will implement the prime number script.

` + "```bash" + `
cat << 'EOF' > primes.py
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
EOF

python3 primes.py

# Signal completion of the feature
agent-bridge feature set req-must-correctly-identify-prime- --status done --passes true
` + "```", nil
	}

	// Heuristic: QA Agent
	if strings.Contains(prompt, "QA checks") || strings.Contains(prompt, "QA AGENT") {
		return `QA checks passed.
` + "```bash" + `
agent-bridge signal QA_PASSED true
` + "```", nil
	}

	// Heuristic: Manager/PM (Approval)
	// Guard: Do not trigger if the prompt looks like it's for a Coding Agent (contains "Implement" or "python" or "script")
	isCodingPrompt := strings.Contains(prompt, "Implement") || strings.Contains(prompt, "python") || strings.Contains(prompt, "script")
	if (strings.Contains(prompt, "Project Manager") || strings.Contains(prompt, "sign off")) && !isCodingPrompt {
		return `Approved.
` + "```bash" + `
agent-bridge signal PROJECT_SIGNED_OFF true --privileged
` + "```", nil
	}

	// Return a mock response that shows the agent received the prompt
	// This allows the session to run without requiring real API keys
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100))

	// Add ls -la as per memory "defaults to returning ls -la"
	response += "\n\n```bash\nls -la\n```"

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
