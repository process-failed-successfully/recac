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
// It returns a mock response that acknowledges the prompt
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// Heuristics for Planning Phase (Ticket Generation)
	// Triggers when prompt asks for a ticket plan (e.g. from TPM persona)
	lowerPrompt := strings.ToLower(prompt)
	if strings.Contains(lowerPrompt, "role - project manager") ||
		strings.Contains(lowerPrompt, "technical program manager") ||
		strings.Contains(lowerPrompt, "ticket generation") {

		// Determine the ticket ID from the prompt if possible
		// The prompt usually asks for a ticket for [PRIMES]
		if strings.Contains(prompt, "[PRIMES]") {
			ticket := map[string]interface{}{
				"id":      "PRIMES",
				"summary": "[PRIMES] Create Prime Number Script",
				"desc":    "Implement a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to a file named 'primes.json'. The JSON format must have a single key 'primes' containing the list of integers.",
				"type":    "Task",
			}
			// Wrap in list as expected by the planner
			tickets := []map[string]interface{}{ticket}
			jsonBytes, _ := json.MarshalIndent(tickets, "", "  ")
			return fmt.Sprintf("```json\n%s\n```", string(jsonBytes)), nil
		}
	}

	// Heuristics for Coding Phase (Implementation)
	// Triggers when prompt asks for implementation (usually contains file names or ID)
	if strings.Contains(lowerPrompt, "prime") ||
		strings.Contains(lowerPrompt, "primes.json") ||
		strings.Contains(lowerPrompt, "id:[primes]") {

		// Return a bash block that creates the python script and runs it
		script := `import json

def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

primes = [n for n in range(2, 10000) if is_prime(n)]

with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
print(f"Generated {len(primes)} primes to primes.json")`

		// Use cat << 'EOF' to avoid shell expansion issues
		response := fmt.Sprintf(`Here is the implementation for the prime number script.

I will create the python file and run it to generate the JSON output.

`+"```bash"+`
cat << 'EOF' > primes.py
%s
EOF

python3 primes.py
git add primes.py primes.json
git commit -m "Add primes.py and generated primes.json"
`+"```"+`
`, script)
		return response, nil
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

// truncateString truncates a string to a maximum length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
