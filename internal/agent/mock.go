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
// It returns a mock response that acknowledges the prompt or returns structured data based on heuristics
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	lowerPrompt := strings.ToLower(prompt)
	injectedFeatures := strings.ToLower(os.Getenv("RECAC_INJECTED_FEATURES"))

	// Heuristic 1: Primes Task (Smoke Test)
	// Triggers if prompt or env var mentions "prime" or specific ID
	if strings.Contains(lowerPrompt, "prime") || strings.Contains(lowerPrompt, "primes.json") ||
		strings.Contains(lowerPrompt, "id:[primes]") || strings.Contains(injectedFeatures, "prime") {
		return `[
  {
    "id": "PRIMES-1",
    "title": "Implement Prime Number Generator",
    "description": "Create a Python script that generates prime numbers up to a given limit.",
    "type": "Task",
    "dependencies": []
  },
  {
    "id": "PRIMES-2",
    "title": "Add Unit Tests for Primes",
    "description": "Write unit tests to verify the prime number generator.",
    "type": "Task",
    "dependencies": ["PRIMES-1"]
  }
]`, nil
	}

	// Heuristic 2: Technical Program Manager (TPM) - General
	// Returns a valid JSON list of tickets to satisfy recac CLI
	if strings.Contains(lowerPrompt, "technical program manager") || strings.Contains(lowerPrompt, "tpm") {
		return `[
  {
    "id": "TASK-1",
    "title": "Initialize Project Structure",
    "description": "Set up the basic project structure and configuration files.",
    "type": "Task",
    "dependencies": []
  },
  {
    "id": "TASK-2",
    "title": "Implement Core Logic",
    "description": "Implement the main functionality of the application.",
    "type": "Task",
    "dependencies": ["TASK-1"]
  }
]`, nil
	}

	// Heuristic 3: QA Agent
	if strings.Contains(lowerPrompt, "qa agent") || strings.Contains(lowerPrompt, "qa_passed") || strings.Contains(lowerPrompt, "you are the qa agent") {
		return "QA_PASSED", nil
	}

	// Heuristic 4: Manager Review
	// Triggers completion of the workflow
	if strings.Contains(lowerPrompt, "manager review") || strings.Contains(lowerPrompt, "project manager") || strings.Contains(lowerPrompt, "qa report") {
		return "agent-bridge signal PROJECT_SIGNED_OFF true --privileged", nil
	}

	// Default: Return a text response
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
