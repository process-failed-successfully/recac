package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Heuristics(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	tests := []struct {
		name           string
		prompt         string
		expectKeywords []string
	}{
		{
			name:           "TPM Phase",
			prompt:         "Act as a Lead Software Architect and generate a JSON plan",
			expectKeywords: []string{"ID:[PRIMES]", "Implement Python Script", "Story"},
		},
		{
			name:           "Coding Phase",
			prompt:         "Implement a python script to calculate primes",
			expectKeywords: []string{"git config", "primes.py", "def is_prime"},
		},
		{
			name:           "Fallback",
			prompt:         "Hello world",
			expectKeywords: []string{"I received your prompt", "Mock agent response"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := agent.Send(ctx, tt.prompt)
			if err != nil {
				t.Fatalf("Send failed: %v", err)
			}
			for _, keyword := range tt.expectKeywords {
				if !strings.Contains(resp, keyword) {
					t.Errorf("Response missing keyword '%s'. Got:\n%s", keyword, resp)
				}
			}
		})
	}
}
