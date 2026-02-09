package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_HeuristicsFormatting(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	tests := []struct {
		name   string
		prompt string
		check  func(t *testing.T, resp string)
	}{
		{
			name:   "Initializer",
			prompt: "ROLE - INITIALIZER AGENT",
			check: func(t *testing.T, resp string) {
				if !strings.Contains(resp, "```bash") {
					t.Error("Initializer response missing markdown block")
				}
				if !strings.Contains(resp, "agent-bridge import") {
					t.Error("Initializer response missing import command")
				}
			},
		},
		{
			name:   "Coding Agent - Primes",
			prompt: "## YOUR ROLE - CODING AGENT\n\nTask: req-primes-py-exists",
			check: func(t *testing.T, resp string) {
				if !strings.Contains(resp, "```bash") {
					t.Error("Coding Agent response missing markdown block")
				}
				if !strings.Contains(resp, "|| true") && !strings.Contains(resp, "|| echo") {
					// We want || true or similar for safety on some commands
					// specifically feature set might fail if ID is wrong
				}
			},
		},
		{
			name:   "QA Agent",
			prompt: "ROLE - QA AGENT",
			check: func(t *testing.T, resp string) {
				if !strings.Contains(resp, "```bash") {
					t.Error("QA Agent response missing markdown block")
				}
				if !strings.Contains(resp, "agent-bridge signal QA_PASSED true") {
					t.Error("QA Agent response missing signal")
				}
			},
		},
		{
			name:   "Project Manager",
			prompt: "ROLE - PROJECT MANAGER",
			check: func(t *testing.T, resp string) {
				if !strings.Contains(resp, "```bash") {
					t.Error("Project Manager response missing markdown block")
				}
				if !strings.Contains(resp, "agent-bridge signal PROJECT_SIGNED_OFF true") {
					t.Error("Project Manager response missing signal")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := agent.Send(ctx, tt.prompt)
			if err != nil {
				t.Fatalf("Send failed: %v", err)
			}
			tt.check(t, resp)
		})
	}
}
