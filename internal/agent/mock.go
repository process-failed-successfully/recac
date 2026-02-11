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
	iterationCount int
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
	m.iterationCount++

	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// TPM (Technical Program Manager) - Ticket Generation
	if strings.Contains(prompt, "Technical Program Manager") {
		tickets := []map[string]interface{}{
			{
				"title":       "Implement Primes",
				"description": "Create a python script that prints prime numbers up to 100",
				"type":        "Task",
			},
		}
		data, _ := json.Marshal(tickets)
		return string(data), nil
	}

	// Initializer Agent - Feature List
	// MUST be before loop breakers to ensure features are created even if prompts contain trigger words
	// Use case-insensitive check for reliability
	if strings.Contains(strings.ToUpper(prompt), "CREATE FEATURE_LIST.JSON") {
		// Return valid JSON structure for agent-bridge import
		// We use a bash block because the system ignores raw JSON blocks
		features := map[string]interface{}{
			"project_name": "Primes",
			"features": []map[string]interface{}{
				{
					"id":          "prime-numbers",
					"category":    "functional",
					"priority":    "MVP",
					"description": "Implement Primes",
					"status":      "pending",
					"steps":       []string{"Verify prime calculation"},
					"passes":      false,
					"dependencies": map[string]interface{}{
						"depends_on_ids":        []string{},
						"exclusive_write_paths": []string{"primes.py"},
						"read_only_paths":       []string{},
					},
				},
			},
		}
		data, _ := json.Marshal(features)
		return fmt.Sprintf(`I will create the feature list.

`+"```bash"+`
cat << 'EOF' | agent-bridge import
%s
EOF
`+"```"+`
`, string(data)), nil
	}

	// Loop Breaker / QA Check
	// If the system says "nothing to commit", it means the previous step (implementation) is done and committed.
	// We should signal success to move forward.
	promptLower := strings.ToLower(prompt)
	if strings.Contains(promptLower, "nothing to commit") || strings.Contains(promptLower, "working tree clean") || strings.Contains(promptLower, "everything up-to-date") {
		return "It looks like the work is done and clean. I will signal completion.\n```bash\nagent-bridge signal --privileged QA_PASSED true\nagent-bridge signal --privileged PROJECT_SIGNED_OFF true\n```", nil
	}

	// Iteration-based Loop Breaker (Fallback)
	// If we've been called many times for the same coding task, assume we are looping and force exit.
	if m.iterationCount > 3 && (strings.Contains(promptLower, "prime numbers") || strings.Contains(prompt, "Implement Primes")) {
		return "It seems I am repeating myself. The work must be done. I will signal completion.\n```bash\nagent-bridge signal --privileged QA_PASSED true\nagent-bridge signal --privileged PROJECT_SIGNED_OFF true\n```", nil
	}

	// Coding Agent - Implementation
	if strings.Contains(prompt, "prime numbers") || strings.Contains(prompt, "Implement Primes") {
		// Return a bash command to create the file, so the agent actually performs an action
		// and avoids tripping the NO-OP loop circuit breaker.
		return `I will create the python script.

` + "```bash" + `
cat <<EOF > primes.py
def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

for i in range(1, 101):
    if is_prime(i):
        print(i)
EOF

# Ensure we commit the changes so the loop breaker can detect "nothing to commit" on subsequent runs
git config user.email "agent@recac.com"
git config user.name "Recac Agent"
git add primes.py
git commit -m "Implement primes" || echo "nothing to commit"
git push || echo "push failed"
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
