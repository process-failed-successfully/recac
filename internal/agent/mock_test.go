package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent(t *testing.T) {
	agent := NewMockAgent()

	prompt := "This is a test prompt that is long enough to be truncated"
	response, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "Mock agent response") {
		t.Errorf("Response missing prefix, got: %s", response)
	}

	if !strings.Contains(response, "I received your prompt") {
		t.Errorf("Response missing body, got: %s", response)
	}
}

func TestTruncateString(t *testing.T) {
	s := "hello world"
	if truncateString(s, 5) != "hello" {
		t.Errorf("Expected 'hello', got '%s'", truncateString(s, 5))
	}
	if truncateString(s, 20) != "hello world" {
		t.Errorf("Expected 'hello world', got '%s'", truncateString(s, 20))
	}
}

func TestMockAgent_Heuristics(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	tests := []struct {
		name        string
		prompt      string
		wantInResp  []string
		avoidInResp []string
	}{
		{
			name:       "Initializer",
			prompt:     "Your role is INITIALIZER AGENT. Please create FEATURE_LIST.JSON for the project involving [PRIMES].",
			wantInResp: []string{"cat <<EOF > feature_list.json", "agent-bridge import < feature_list.json", "req-primes"},
		},
		{
			name:       "TPM",
			prompt:     "You are the TECHNICAL PROGRAM MANAGER. Create tickets for [PRIMES].",
			wantInResp: []string{`"id": "PRIMES"`, `"title": "ID:[PRIMES] Create Prime Number Script"`},
		},
		{
			name:       "Coding Agent",
			prompt:     "You are the CODING AGENT. Implement the task for [PRIMES]. Ensure primes.py is created.",
			wantInResp: []string{"cat << 'EOF' > primes.py", "python3 primes.py", "git add primes.py", "agent-bridge feature set req-primes --status done"},
		},
		{
			name:       "QA Agent",
			prompt:     "You are the QA AGENT. Verify the implementation of [PRIMES].",
			wantInResp: []string{"agent-bridge signal --privileged QA_PASSED true"},
			avoidInResp: []string{"cat << 'EOF' > primes.py"}, // Should NOT re-implement code!
		},
		{
			name:       "Project Manager",
			prompt:     "You are the PROJECT MANAGER. Review and sign off.",
			wantInResp: []string{"agent-bridge signal --privileged PROJECT_SIGNED_OFF true"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := agent.Send(ctx, tt.prompt)
			if err != nil {
				t.Fatalf("Send failed: %v", err)
			}
			for _, want := range tt.wantInResp {
				if !strings.Contains(resp, want) {
					t.Errorf("Response missing expected string %q. Got:\n%s", want, resp)
				}
			}
			for _, avoid := range tt.avoidInResp {
				if strings.Contains(resp, avoid) {
					t.Errorf("Response contains avoided string %q. Got:\n%s", avoid, resp)
				}
			}
		})
	}
}
