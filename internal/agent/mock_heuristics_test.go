package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Send_Heuristics(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	tests := []struct {
		name           string
		prompt         string
		expectContains []string
	}{
		{
			name:   "Implementation - COMP-1",
			prompt: "Please Implement Core Feature (COMP-1)",
			expectContains: []string{
				"Mock agent implementation for COMP-1",
				"```bash",
				"echo \"Implementing COMP-1 Core Feature...\"",
				"agent-bridge feature set req-feature-works --status done",
			},
		},
		{
			name:   "Implementation - req-feature-works",
			prompt: "Work on req-feature-works",
			expectContains: []string{
				"Mock agent implementation for COMP-1",
				"agent-bridge feature set req-feature-works --status done",
			},
		},
		{
			name:   "QA Agent",
			prompt: "You are the QA Agent. Verify functionality.",
			expectContains: []string{
				"Mock QA agent response",
				"agent-bridge signal QA_PASSED true",
			},
		},
		{
			name:   "Project Manager",
			prompt: "You are the Project Manager. Review the project.",
			expectContains: []string{
				"Mock Manager response",
				"agent-bridge signal PROJECT_SIGNED_OFF true",
			},
		},
		{
			name:   "Fallback",
			prompt: "Hello world",
			expectContains: []string{
				"I received your prompt",
				"In mock mode",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := agent.Send(ctx, tt.prompt)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			for _, exp := range tt.expectContains {
				if !strings.Contains(resp, exp) {
					t.Errorf("expected response to contain %q, got: \n%s", exp, resp)
				}
			}
		})
	}
}
