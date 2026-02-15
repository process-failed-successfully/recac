package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_QA_Heuristic(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	tests := []struct {
		name        string
		prompt      string
		expected    string
		notExpected string
	}{
		{
			name:     "QA Agent - Explicit Identification",
			prompt:   "You are the QA Agent. Please verify the changes.",
			expected: "QA_PASSED",
		},
		{
			name:        "QA Agent - False Positive in System Prompt",
			prompt:      "You are the Coding Agent. The QA Agent will review your work later.",
			expected:    "Mock agent response", // Should NOT be QA_PASSED
			notExpected: "QA_PASSED",
		},
		{
			name:     "QA Agent - Explicit Check via qa_passed",
			prompt:   "Is the build qa_passed?",
			expected: "QA_PASSED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := agent.Send(ctx, tt.prompt)
			if err != nil {
				t.Fatalf("Send failed: %v", err)
			}

			if tt.expected != "" && !strings.Contains(resp, tt.expected) {
				t.Errorf("Expected output to contain %q, got:\n%s", tt.expected, resp)
			}

			if tt.notExpected != "" && strings.Contains(resp, tt.notExpected) {
				t.Errorf("Expected output NOT to contain %q, but it did:\n%s", tt.notExpected, resp)
			}
		})
	}
}
