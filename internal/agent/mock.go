package agent

import (
	"context"
	"fmt"
	"regexp"
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

	// 1. Technical Program Manager (TPM) - Generate Tickets
	if strings.Contains(prompt, "Technical Program Manager") {
		return m.handleTPM(prompt), nil
	}

	// 2. QA Agent
	if strings.Contains(prompt, "QA AGENT") {
		return "```bash\nagent-bridge signal QA_PASSED true\n```", nil
	}

	// 3. Project Manager
	if strings.Contains(prompt, "PROJECT MANAGER") {
		return "```bash\nagent-bridge signal PROJECT_SIGNED_OFF true\n```", nil
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

func (m *MockAgent) handleTPM(prompt string) string {
	// Extract ID:[...] from prompt
	re := regexp.MustCompile(`ID:\[(.*?)\]`)
	matches := re.FindStringSubmatch(prompt)

	idTag := "EXAMPLE"
	if len(matches) > 1 {
		idTag = matches[1]
	}
	fullTag := fmt.Sprintf("ID:[%s]", idTag)

	// Return valid JSON tickets
	// Note: We exclude Repo: url here so that the CLI can inject the correct one from flags.

	// Construct JSON using standard string literals to avoid backtick issues
	jsonBody := fmt.Sprintf(`[
  {
    "title": "%s Feature Implementation",
    "description": "Implementation of the requested feature.",
    "type": "Epic",
    "children": [
      {
        "title": "%s Core Logic",
        "description": "Implement the core logic.",
        "type": "Story",
        "acceptance_criteria": [
          "Logic is implemented",
          "Tests pass"
        ]
      }
    ]
  }
]`, fullTag, fullTag)

	return fmt.Sprintf("```json\n%s\n```", jsonBody)
}

// truncateString truncates a string to a maximum length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
