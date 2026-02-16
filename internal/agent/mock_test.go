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

func TestMockAgent_Send_Heuristics(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	tests := []struct {
		name      string
		prompt    string
		wantInResp string
	}{
		{
			name:      "Planning Phase",
			prompt:    "You are an expert Technical Program Manager",
			wantInResp: `"id": "PRIMES-1"`,
		},
		{
			name:      "Planning Phase - Correct Acceptance Criteria",
			prompt:    "You are an expert Technical Program Manager",
			wantInResp: `"acceptance_criteria": ["script-runs-without-errors"]`,
		},
		{
			name:      "Coding Phase - Primes",
			prompt:    "Here is the task: ID:[PRIMES] Create a python script",
			wantInResp: "range(10000)",
		},
		{
			name:      "Coding Phase - Primes Summary",
			prompt:    "Task: Implement Prime Number Script",
			wantInResp: "range(10000)",
		},
		{
			name:      "Planning Phase - Primes Spec (Real)",
			prompt:    "You are an expert Technical Program Manager. ... Task: Implement Prime Number Script",
			wantInResp: "PRIMES-1",
		},
		{
			name:      "QA Phase",
			prompt:    "Your role - QA Agent. Please review.",
			wantInResp: "agent-bridge signal PROJECT_SIGNED_OFF true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := agent.Send(ctx, tt.prompt)
			if err != nil {
				t.Errorf("Send() error = %v", err)
				return
			}
			if !strings.Contains(got, tt.wantInResp) {
				t.Errorf("Send() = %v, want substring %v", got, tt.wantInResp)
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
