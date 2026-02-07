package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Scenarios(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	tests := []struct {
		name           string
		prompt         string
		wantContains   []string
		wantNotContain []string
	}{
		{
			name:   "Initializer",
			prompt: "You are the INITIALIZER AGENT.",
			wantContains: []string{
				"agent-bridge import",
				"req-primes-py-exists",
			},
		},
		{
			name:   "TPM Primes",
			prompt: "You are the Technical Program Manager... [PRIMES]",
			wantContains: []string{
				`"tickets":`,
				`"title": "ID:[PRIMES] Create primes.py"`,
			},
			wantNotContain: []string{
				"Mock agent response", // Should not be fallback
			},
		},
		{
			name:   "Developer Primes",
			prompt: "Task: Create primes.py [PRIMES]",
			wantContains: []string{
				"```bash",
				"cat << 'EOF' > primes.py",
				"agent-bridge feature set req-primes-py-exists --status done --passes true",
			},
		},
		{
			name:   "QA Agent",
			prompt: "You are the QA AGENT",
			wantContains: []string{
				"agent-bridge signal QA_PASSED true",
			},
		},
		{
			name:   "Project Manager",
			prompt: "You are the PROJECT MANAGER. Signal PROJECT_SIGNED_OFF set to true",
			wantContains: []string{
				"echo \"Project Sign Off\"",
			},
		},
		{
			name:   "Fallback",
			prompt: "Hello world",
			wantContains: []string{
				"Mock agent response",
				"I received your prompt",
				"```bash",
				"echo \"Mock Agent: Processing",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := agent.Send(ctx, tt.prompt)
			if err != nil {
				t.Fatalf("Send() error = %v", err)
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("Send() missing %q in response:\n%s", want, got)
				}
			}

			for _, notWant := range tt.wantNotContain {
				if strings.Contains(got, notWant) {
					t.Errorf("Send() unexpectedly contained %q:\n%s", notWant, got)
				}
			}
		})
	}
}
