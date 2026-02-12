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
		name        string
		prompt      string
		wantContain []string
	}{
		{
			name:   "TPM Logic",
			prompt: "You are a TPM. Repo: https://github.com/test/repo",
			wantContain: []string{
				"```json",
				`"project_key": "MFLP"`,
				`"stories": [`,
				`ID:[PRIMES-SCRIPT]`,
				"https://github.com/test/repo",
			},
		},
		{
			name:   "Coding Agent Logic",
			prompt: "Create primes.py",
			wantContain: []string{
				"```bash",
				"cat <<EOF > primes.py",
				"def is_prime(n):",
				"git add primes.py",
				"agent-bridge feature set PRIMES-SCRIPT",
			},
		},
		{
			name:   "Initializer Agent Logic",
			prompt: "You are the Initializer Agent",
			wantContain: []string{
				"```bash",
				"agent-bridge import --file -",
				`{"features": [{"id": "PRIMES-SCRIPT"`,
			},
		},
		{
			name:   "QA Agent Logic",
			prompt: "Approve or Reject the changes",
			wantContain: []string{
				"```bash",
				"agent-bridge signal QA_PASSED true --privileged",
			},
		},
		{
			name:   "Generic Logic",
			prompt: "Hello world",
			wantContain: []string{
				"Mock agent response",
				"Prompt preview: Hello world...",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := agent.Send(ctx, tt.prompt)
			if err != nil {
				t.Fatalf("Send failed: %v", err)
			}

			for _, want := range tt.wantContain {
				if !strings.Contains(resp, want) {
					t.Errorf("Response missing %q. Got:\n%s", want, resp)
				}
			}
		})
	}
}

func TestMockAgent_SendStream(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()
	prompt := "Hello"

	var streamed string
	resp, err := agent.SendStream(ctx, prompt, func(chunk string) {
		streamed += chunk
	})

	if err != nil {
		t.Fatalf("SendStream failed: %v", err)
	}

	if resp != streamed {
		t.Errorf("Streamed content mismatch. Got %q, want %q", streamed, resp)
	}

	if !strings.Contains(resp, "Mock agent response") {
		t.Errorf("Response missing prefix")
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
