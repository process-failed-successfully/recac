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

func TestMockAgent_Heuristics(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	tests := []struct {
		name           string
		prompt         string
		expectContains []string
		expectMissing  []string
	}{
		{
			name:           "Planning Phase - Ticket Generation",
			// Updated to reflect actual prompt template (Technical Program Manager)
			prompt:         "You are an expert Technical Program Manager (TPM)\nPlease generate tickets",
			expectContains: []string{`"tickets": [`, "Implement Prime Number Script"},
			expectMissing:  []string{"PROJECT_SIGNED_OFF"},
		},
		{
			name:           "Coding Phase - Primes",
			prompt:         "Implement the prime number script",
			expectContains: []string{"cat <<EOF > primes.py", "python3 primes.py", "PROJECT_SIGNED_OFF"},
			expectMissing:  []string{"Based on the QA Report"},
		},
		{
			name:           "Coding Phase - Precedence over Manager",
			// This prompt simulates a mixed signal but lacks "QA Report", so it falls through to Coding
			prompt:         "## your role - project manager\n\nImplement Prime Number Script",
			expectContains: []string{"cat <<EOF > primes.py", "python3 primes.py"},
			// Should NOT be the simple sign-off message
			expectMissing: []string{"Based on the QA Report, I approve the project"},
		},
		{
			name:           "Manager Review - Sign Off",
			// Must include "QA Report" to trigger Manager Review now
			prompt:         "## your role - project manager\nReview the QA report.",
			expectContains: []string{"Based on the QA Report, I approve the project", "PROJECT_SIGNED_OFF"},
			expectMissing:  []string{"cat <<EOF > primes.py"},
		},
		{
			name:           "Planning Phase - Primes Spec",
			// This prompt contains "Implement Prime" (Coding heuristic) AND "TPM" (Planning heuristic)
			// It does NOT contain "generate tickets", ensuring that TPM role takes precedence even without that keyword
			prompt:         "Technical Program Manager\nImplement Prime Number Script",
			expectContains: []string{`"tickets": [`, "Implement Prime Number Script"},
			expectMissing:  []string{"cat <<EOF > primes.py"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := agent.Send(ctx, tt.prompt)
			if err != nil {
				t.Fatalf("Send failed: %v", err)
			}

			for _, exp := range tt.expectContains {
				if !strings.Contains(resp, exp) {
					t.Errorf("Expected response to contain %q, but it didn't.\nResponse: %s", exp, resp)
				}
			}

			for _, missing := range tt.expectMissing {
				if strings.Contains(resp, missing) {
					t.Errorf("Expected response NOT to contain %q, but it did.\nResponse: %s", missing, resp)
				}
			}
		})
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
