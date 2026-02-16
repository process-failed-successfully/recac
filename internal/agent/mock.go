package agent

import (
	"context"
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
// It returns a mock response that acknowledges the prompt
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	promptLower := strings.ToLower(prompt)
	injectedFeatures := strings.ToLower(os.Getenv("RECAC_INJECTED_FEATURES"))

	// Heuristic: Technical Program Manager (TPM) - Return JSON tickets
	if strings.Contains(prompt, "Technical Program Manager") {
		return `[
  {
    "title": "ID:[PRIMES] Implement Prime Number Generator",
    "description": "Create a Python script that generates prime numbers.",
    "type": "Story",
    "children": []
  }
]`, nil
	}

	// Heuristic: Primes Coding Task
	if strings.Contains(promptLower, "prime") || strings.Contains(injectedFeatures, "prime") ||
		strings.Contains(promptLower, "primes.json") || strings.Contains(injectedFeatures, "primes.json") ||
		strings.Contains(promptLower, "id:[primes]") || strings.Contains(injectedFeatures, "id:[primes]") {

		return "Done. I have implemented the prime number generator in `primes.py`.", nil
	}

	// Heuristic: QA Agent
	if strings.Contains(prompt, "You are the QA Agent") || strings.Contains(promptLower, "qa_passed") {
		return "QA_PASSED", nil
	}

	// Heuristic: Manager Review
	if strings.Contains(promptLower, "qa report") || strings.Contains(promptLower, "## your role - project manager") {
		return "agent-bridge signal PROJECT_SIGNED_OFF true --privileged", nil
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
