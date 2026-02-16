package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
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
// It returns a mock response that acknowledges the prompt or acts on heuristics
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	lowerPrompt := strings.ToLower(prompt)
	injectedFeatures := strings.ToLower(os.Getenv("RECAC_INJECTED_FEATURES"))

	// 1. Heuristic: Planning Phase (TPM)
	// Trigger if prompt mentions "Technical Program Manager" or "TPM"
	if strings.Contains(lowerPrompt, "technical program manager") || strings.Contains(lowerPrompt, "tpm") {
		// Return a valid JSON list of tickets
		tickets := []map[string]string{
			{
				"id":          "PRIMES",
				"title":       "Create Prime Number Script",
				"description": "Create a python script that calculates primes up to 10000 and saves them to primes.json. Then output the results.",
				"type":        "Task",
				"assigned_to": "Agent",
			},
		}
		jsonBytes, _ := json.Marshal(tickets)
		return fmt.Sprintf("Here is the plan:\n```json\n%s\n```", string(jsonBytes)), nil
	}

	// 2. Heuristic: Coding Phase (Primes)
	// Trigger if prompt or env contains "prime" or "primes.json"
	// BUT exclude if it looks like a Manager Review (which might also mention the task name)
	if (strings.Contains(lowerPrompt, "prime") || strings.Contains(injectedFeatures, "prime")) &&
		!strings.Contains(lowerPrompt, "manager review") &&
		!strings.Contains(lowerPrompt, "qa agent") {

		script := `
# Create the python script
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n < 2:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

primes = [x for x in range(10000) if is_prime(x)]

with open('primes.json', 'w') as f:
    json.dump({'primes': primes}, f)
EOF

# Run the script to generate output
python3 primes.py

# Commit changes
git add primes.py primes.json
git commit -m "Implement primes script"

# Signal completion
agent-bridge signal COMPLETED true
`
		return fmt.Sprintf("I will implement the prime number script.\n```bash\n%s\n```", script), nil
	}

	// 3. Heuristic: QA Phase
	// Trigger if prompt mentions "QA AGENT" or "QA Agent"
	if strings.Contains(lowerPrompt, "qa agent") || strings.Contains(lowerPrompt, "qa_passed") {
		return "Tests passed.\n```bash\nagent-bridge signal QA_PASSED true\n```", nil
	}

	// 4. Heuristic: Manager Review
	// Trigger if prompt mentions "Project Manager" or "Manager Review"
	if strings.Contains(lowerPrompt, "project manager") || strings.Contains(lowerPrompt, "manager review") {
		return "Project approved.\n```bash\nagent-bridge signal PROJECT_SIGNED_OFF true --privileged\n```", nil
	}

	// Default Response
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
