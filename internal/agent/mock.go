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

	// Heuristics for E2E Smoke Tests
	upperPrompt := strings.ToUpper(prompt)

	// 1. Initializer Role (Feature List)
	if strings.Contains(upperPrompt, "INITIALIZER") || strings.Contains(upperPrompt, "FEATURE LIST") {
		return "```bash\ncat << 'EOF' | agent-bridge import --features\n{\"features\": [{\"id\": \"1\", \"description\": \"Implement Primes\", \"status\": \"todo\"}, {\"id\": \"2\", \"description\": \"Review Primes\", \"status\": \"todo\"}]}\nEOF\n```", nil
	}

	// 2. TPM Role (Ticket Generation)
	if strings.Contains(upperPrompt, "PROGRAM MANAGER") || (strings.Contains(upperPrompt, "GENERATE") && strings.Contains(upperPrompt, "TICKET")) {
		return "```json\n[{\"title\": \"ID:[MFLP-11113] Implement Primes\", \"description\": \"Implement a function to check for prime numbers.\", \"type\": \"Task\"}]\n```", nil
	}

	// 3. Coding Role (Primes Task)
	if strings.Contains(upperPrompt, "PRIME") || (strings.Contains(upperPrompt, "CODING AGENT") && strings.Contains(upperPrompt, "PYTHON")) {
		return "```bash\ncat << 'EOF' > primes.py\ndef is_prime(n):\n    if n <= 1: return False\n    for i in range(2, int(n**0.5) + 1):\n        if n % i == 0: return False\n    return True\n\nprint([x for x in range(20) if is_prime(x)])\nEOF\n\npython3 primes.py\n```", nil
	}

	// 4. QA/Manager Role (Review & Signoff)
	// We check for "QA", "REVIEW", or "MANAGER" but ensure we don't trigger on "CODING AGENT" context
	if (strings.Contains(upperPrompt, "QA") || strings.Contains(upperPrompt, "REVIEW") || strings.Contains(upperPrompt, "MANAGER")) && !strings.Contains(upperPrompt, "CODING AGENT") {
		// Return a privileged signal to sign off the project
		return "```bash\nagent-bridge signal --privileged PROJECT_SIGNED_OFF\n```", nil
	}

	// Default: Return a generic response that allows the loop to continue (or trigger circuit breaker if empty)
	// For unit tests expecting text, we return text.
	// For loop tests expecting NoOp, we should perhaps return empty?
	// But `NewMockAgent` is used in many tests expecting content.
	// Let's keep the generic response but keep it simple.
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response.",
		m.responsePrefix, len(prompt))
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
