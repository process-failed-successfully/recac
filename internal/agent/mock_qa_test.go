package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Logic(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	tests := []struct {
		name           string
		prompt         string
		expectedSubstr []string
	}{
		{
			name:   "Implementation Request",
			prompt: "Implementation Request: Create primes.py to calculate primes.",
			expectedSubstr: []string{
				"python3 primes.py",
				"agent-bridge feature set --id primes-impl --status done --passes true",
			},
		},
		{
			name:   "QA Agent Request",
			prompt: "## YOUR ROLE - QA AGENT\nVerify the implementation.",
			expectedSubstr: []string{
				"agent-bridge signal set QA_PASSED true",
				"QA checks passed",
			},
		},
		{
			name:   "Manager Request",
			prompt: "## YOUR ROLE - PROJECT MANAGER\nDecide if project is approved.",
			expectedSubstr: []string{
				"agent-bridge signal set PROJECT_SIGNED_OFF true",
				"Project approved",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := agent.Send(ctx, tt.prompt)
			if err != nil {
				t.Fatalf("Send failed: %v", err)
			}
			for _, substr := range tt.expectedSubstr {
				if !strings.Contains(resp, substr) {
					t.Errorf("Response missing expected substring '%s'. Got:\n%s", substr, resp)
				}
			}
		})
	}
}
