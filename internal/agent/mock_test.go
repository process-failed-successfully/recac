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
		expectedScript string
		expectedText   string
	}{
		{
			name:           "Manager Review",
			prompt:         "Role: Manager. Review the QA Report. Approve if good.",
			expectedScript: "agent-bridge signoff",
			expectedText:   "I approve the project",
		},
		{
			name:           "Implementation",
			prompt:         "Role: Coding Agent. Task: [PRIMES]. Create primes.py.",
			expectedScript: "agent-bridge feature set 1",
			expectedText:   "implement the prime number script",
		},
		{
			name:           "Initializer",
			prompt:         "Role: INITIALIZER. Initialize feature_list.json.",
			expectedScript: "> feature_list.json",
			expectedText:   "create the feature list",
		},
		{
			name:           "Recovery - Nothing to Commit",
			prompt:         "Command Failed: git commit ... Output: nothing to commit",
			expectedScript: "agent-bridge signal QA_PASSED true",
			expectedText:   "mark the feature as complete",
		},
		{
			name:           "False Positive Initializer (Coding Agent)",
			prompt:         "Role: Coding Agent. Implement feature from feature_list.json. Create primes.py.",
			expectedScript: "agent-bridge feature set 1", // Should trigger Implementation, NOT Initializer
			expectedText:   "implement the prime number script",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := agent.Send(ctx, tc.prompt)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tc.expectedScript != "" && !strings.Contains(resp, tc.expectedScript) {
				t.Errorf("Expected script part %q not found in response:\n%s", tc.expectedScript, resp)
			}

			if tc.expectedText != "" && !strings.Contains(resp, tc.expectedText) {
				t.Errorf("Expected text part %q not found in response:\n%s", tc.expectedText, resp)
			}
		})
	}
}
