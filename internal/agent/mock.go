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

	// Heuristics for different roles/scenarios
	promptLower := strings.ToLower(prompt)
	injectedFeatures := strings.ToLower(os.Getenv("RECAC_INJECTED_FEATURES"))

	// 1. Technical Program Manager (TPM) - Returns JSON list of tasks
	if strings.Contains(promptLower, "technical program manager") || strings.Contains(promptLower, "tpm") {
		return `[{"id":"TASK-1","summary":"Implement core logic","description":"Implement the core business logic","status":"todo","type":"task","assignee":"recac-agent"},{"id":"TASK-2","summary":"Add tests","description":"Add unit tests","status":"todo","type":"task","assignee":"recac-agent"}]`, nil
	}

	// 2. Primes Coding Task - Returns JSON object with primes
	if strings.Contains(promptLower, "prime") || strings.Contains(promptLower, "primes.json") || strings.Contains(injectedFeatures, "primes.json") || strings.Contains(injectedFeatures, "id:[primes]") {
		return `{"primes": [2, 3, 5, 7, 11, 13, 17, 19, 23, 29]}`, nil
	}

	// 3. QA Agent - Returns QA_PASSED
	if strings.Contains(promptLower, "you are the qa agent") || strings.Contains(promptLower, "qa_passed") {
		return "QA_PASSED", nil
	}

	// 4. Manager Review - Returns PROJECT_SIGNED_OFF signal
	if strings.Contains(promptLower, "qa report") || strings.Contains(promptLower, "## your role - project manager") || strings.Contains(promptLower, "manager review") {
		return "agent-bridge signal PROJECT_SIGNED_OFF true --privileged", nil
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
