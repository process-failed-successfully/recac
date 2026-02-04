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
// It returns a mock response that acknowledges the prompt
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// Debugging Heuristics
	if len(prompt) > 200 {
		fmt.Printf("[MockAgent] Prompt (truncated): %s...\n", prompt[:200])
	} else {
		fmt.Printf("[MockAgent] Prompt: %s\n", prompt)
	}

	// Heuristic: Check for "TPM" or "Technical Program Manager" role to generate a ticket plan
	if strings.Contains(prompt, "TPM") || strings.Contains(prompt, "Technical Program Manager") {
		// Return a valid JSON response for ticket generation (Array of tickets)
		return `[
    {
      "title": "Implement Core Feature",
      "description": "Implement the core functionality as requested.",
      "type": "Epic",
      "children": [
        {
          "title": "Setup Project Structure",
          "description": "Initialize the project structure.",
          "type": "Story"
        },
        {
          "title": "Implement Logic",
          "description": "Write the business logic.",
          "type": "Story"
        }
      ]
    }
  ]`, nil
	}

	promptUpper := strings.ToUpper(prompt)

	// Heuristic: Check if this is the Initializer agent
	// Checking specifically for "INITIALIZER AGENT" role to avoid overlap with Coding Agent which also sees feature_list.json
	if strings.Contains(promptUpper, "YOUR ROLE - INITIALIZER AGENT") {
		// We return a non-empty feature list wrapped in the expected JSON structure
		return "Mock Initializer: Creating feature list.\n```bash\necho '{\"features\": [{\"id\": \"init-task\", \"description\": \"Initialize project\"}]}' > feature_list.json\n```", nil
	}

	// Heuristic: Manager Agent (Check first to avoid QA overlap)
	// We check for "PROJECT MANAGER" in the role header
	if strings.Contains(promptUpper, "YOUR ROLE - PROJECT MANAGER") {
		return "Mock Manager: Project approved.\n```bash\nagent-bridge signal set PROJECT_SIGNED_OFF true\n```", nil
	}

	// Heuristic: QA Agent
	if strings.Contains(promptUpper, "YOUR ROLE - QA AGENT") {
		return "Mock QA: All checks passed.\n```bash\nagent-bridge signal set QA_PASSED true\n```", nil
	}

	// Heuristic: Coding Agent
	if strings.Contains(promptUpper, "YOUR ROLE - CODING AGENT") {
		// Attempt to extract task ID from prompt
		// Prompt format: "**Feature ID**: {task_id}"
		taskID := "init-task" // Default fallback

		lines := strings.Split(prompt, "\n")
		for _, line := range lines {
			if strings.Contains(line, "**Feature ID**:") {
				parts := strings.Split(line, ":")
				if len(parts) >= 2 {
					taskID = strings.TrimSpace(parts[1])
				}
				break
			}
		}

		// Complete the assigned task
		return fmt.Sprintf("Mock Coding Agent: Completed task %s.\n```bash\nagent-bridge feature set %s --status done --passes true\n```", taskID, taskID), nil
	}

	// Return a mock response that shows the agent received the prompt
	// This allows the session to run without requiring real API keys
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...\n\n```bash\necho 'Mock Agent: Processing request...'\n```",
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
