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

	// Heuristics for E2E scenarios
	lowerPrompt := strings.ToLower(prompt)

	// 1. Execution Phase (Coding Agent)
	// We prioritize the explicit role "coding agent" to prevent false positives from history
	if strings.Contains(lowerPrompt, "coding agent") {
		// Prime Python Scenario
		if strings.Contains(lowerPrompt, "prime") {
			return `I will create a python script to calculate primes.

` + "```bash" + `
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = []
for i in range(10000):
    if is_prime(i):
        primes.append(i)

with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)

print(f"Found {len(primes)} primes")
EOF

git add primes.py primes.json
git commit -m "Add primes script"
git push origin HEAD
` + "```", nil
		}
	}

	// 2. QA Agent
	if strings.Contains(lowerPrompt, "qa agent") {
		return "```bash\nagent-bridge signal QA_PASSED true\n```", nil
	}

	// 3. Project Manager
	if strings.Contains(lowerPrompt, "project manager") {
		return "```bash\nagent-bridge signal PROJECT_SIGNED_OFF true --privileged\n```", nil
	}

	// 4. Ticket Planning Phase (TPM Role)
	// Trigger on "technical program manager" or "generate ticket"
	if strings.Contains(lowerPrompt, "technical program manager") || strings.Contains(lowerPrompt, "generate ticket") {
		// Extract repo URL from prompt if possible, or use a placeholder
		repoURL := "https://github.com/example/repo"
		if parts := strings.Split(prompt, "Repo: "); len(parts) > 1 {
			repoURL = strings.Split(parts[1], "\n")[0]
		}

		return fmt.Sprintf(`[
  {
    "summary": "Implement Prime Number Python Script",
    "description": "Create a python script named primes.py that calculates primes up to 10000. \n\nRepo: %s\n\nAppSpec:\nruntime: python\n...",
    "type": "Task",
    "id": "[PRIMES]"
  }
]`, repoURL), nil
	}

	// 5. Fallback Execution (Legacy/Unit Tests without Role Header)
	// We check for "prime" (and optionally "python" though we relaxed it) to support unit tests
	if strings.Contains(lowerPrompt, "prime") {
		return `I will create a python script to calculate primes.

` + "```bash" + `
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = []
for i in range(10000):
    if is_prime(i):
        primes.append(i)

with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)

print(f"Found {len(primes)} primes")
EOF

git add primes.py primes.json
git commit -m "Add primes script"
git push origin HEAD
` + "```", nil
	}

	// Return a mock response that shows the agent received the prompt
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
