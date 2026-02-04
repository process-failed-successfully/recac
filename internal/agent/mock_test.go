package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Send(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	tests := []struct {
		name           string
		prompt         string
		wantContains   string
		wantNotContains string
	}{
		{
			name:         "TPM Agent",
			prompt:       "You are a Technical Program Manager. ID:[PRIMES]",
			wantContains: "ID:[PRIMES] Mock Task",
		},
		{
			name:         "Initializer Agent",
			prompt:       "Initialize feature_list.json",
			wantContains: "cat << 'EOF' | agent-bridge import",
		},
		{
			name:         "QA Agent",
			prompt:       "## YOUR ROLE - QA AGENT",
			wantContains: "agent-bridge signal QA_PASSED true",
		},
		{
			name:         "Project Manager",
			prompt:       "## YOUR ROLE - PROJECT MANAGER",
			wantContains: "agent-bridge signal PROJECT_SIGNED_OFF true",
		},
		{
			name:         "Coding Agent",
			prompt:       "Write a script primes.py",
			wantContains: "agent-bridge feature set req-primes-py-exists",
		},
		{
			name:         "Generic Fallback",
			prompt:       "Hello Agent",
			wantContains: "I received your prompt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := agent.Send(ctx, tt.prompt)
			if err != nil {
				t.Fatalf("Send() error = %v", err)
			}
			if !strings.Contains(got, tt.wantContains) {
				t.Errorf("Send() = %v, want to contain %v", got, tt.wantContains)
			}
			if tt.wantNotContains != "" {
				if strings.Contains(got, tt.wantNotContains) {
					t.Errorf("Send() = %v, want NOT to contain %v", got, tt.wantNotContains)
				}
			}
		})
	}
}
