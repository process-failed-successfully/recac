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
		wantSubstring  string
		wantBashScript bool
	}{
		{
			name:           "Initializer",
			prompt:         "Please Initialize the project and create feature_list.json",
			wantSubstring:  "cat << 'EOF' > feature_list.json",
			wantBashScript: true,
		},
		{
			name:           "Prime Implementation",
			prompt:         "Implement req-primes: create primes.py",
			wantSubstring:  "cat << 'EOF' > primes.py",
			wantBashScript: true,
		},
		{
			name:           "Prime Implementation (Description Only)",
			prompt:         "Title: Primes Script\nDescription: Calculate primes < 10000",
			wantSubstring:  "cat << 'EOF' > primes.py",
			wantBashScript: true,
		},
		{
			name:           "QA Agent",
			prompt:         "You are the QA AGENT. Verify changes.",
			wantSubstring:  "agent-bridge signal QA_PASSED true",
			wantBashScript: true,
		},
		{
			name:           "Project Manager",
			prompt:         "You are the PROJECT MANAGER. Review status.",
			wantSubstring:  "agent-bridge signal PROJECT_SIGNED_OFF true",
			wantBashScript: true,
		},
		{
			name:           "Idempotency Check",
			prompt:         "git status\nnothing to commit, working tree clean",
			wantSubstring:  "agent-bridge update --status done",
			wantBashScript: true,
		},
		{
			name:           "Generate Plan",
			prompt:         "You are the technical program manager. generate-from-spec",
			wantSubstring:  `"id": "req-primes"`,
			wantBashScript: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := agent.Send(ctx, tt.prompt)
			if err != nil {
				t.Fatalf("Send() error = %v", err)
			}
			if !strings.Contains(got, tt.wantSubstring) {
				t.Errorf("Send() got = %v, want substring %v", got, tt.wantSubstring)
			}
			if tt.wantBashScript {
				if !strings.Contains(got, "```bash") {
					t.Errorf("Send() expected bash block")
				}
			} else if tt.name == "Generate Plan" {
                if !strings.Contains(got, "```json") {
					t.Errorf("Send() expected json block")
				}
            }
		})
	}
}
