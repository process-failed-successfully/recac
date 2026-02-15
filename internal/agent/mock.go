package agent

import (
	"context"
	"encoding/json"
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

	lowerPrompt := strings.ToLower(prompt)

	// Heuristic 1: TPM / Ticket Generation for Prime Python Scenario
	// Prompt contains "json" AND ("technical program manager" OR "architect" OR "tpm")
	if strings.Contains(lowerPrompt, "json") &&
		(strings.Contains(lowerPrompt, "technical program manager") ||
			strings.Contains(lowerPrompt, "architect") ||
			strings.Contains(lowerPrompt, "tpm")) {

		// Check if it's the Prime Python scenario (contains [PRIMES])
		if strings.Contains(prompt, "[PRIMES]") {
			// Extract Repo URL from prompt if possible, otherwise use a placeholder or the one from memory
			// The prompt contains "Repo: https://..."
			repoURL := "https://github.com/process-failed-successfully/recac-jira-e2e"
			if start := strings.Index(prompt, "Repo: "); start != -1 {
				rest := prompt[start+6:]
				if end := strings.IndexAny(rest, "\n\r"); end != -1 {
					repoURL = strings.TrimSpace(rest[:end])
				} else {
					repoURL = strings.TrimSpace(rest)
				}
			}

			tickets := []map[string]interface{}{
				{
					"title":       "ID:[PRIMES] Create Prime Number Script",
					"description": fmt.Sprintf("Create a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to 'primes.json'.\nRepo: %s", repoURL),
					"type":        "Task",
					"children":    []interface{}{},
				},
			}
			data, _ := json.Marshal(tickets)
			return fmt.Sprintf("```json\n%s\n```", string(data)), nil
		}
	}

	// Heuristic 2: Coding Phase for Prime Python Scenario
	// Prompt contains "id:[primes]" OR "generate primes" OR "primes.json" OR ("prime" AND "python")
	// AND NOT "technical program manager"
	isCoding := strings.Contains(lowerPrompt, "id:[primes]") ||
		strings.Contains(lowerPrompt, "generate primes") ||
		strings.Contains(lowerPrompt, "primes.json") ||
		(strings.Contains(lowerPrompt, "prime") && strings.Contains(lowerPrompt, "python"))

	if isCoding && !strings.Contains(lowerPrompt, "technical program manager") {
		return m.generatePrimePythonResponse(), nil
	}

	// Default Mock Response
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

func (m *MockAgent) generatePrimePythonResponse() string {
	return `
I will create the 'primes.py' script to calculate prime numbers less than 10,000 and output them to 'primes.json'.

` + "```bash" + `
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

primes = [x for x in range(2, 10000) if is_prime(x)]

with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)

print(f"Generated {len(primes)} primes to primes.json")
EOF

# Run the script to generate the json file
python3 primes.py
` + "```" + `
`
}
