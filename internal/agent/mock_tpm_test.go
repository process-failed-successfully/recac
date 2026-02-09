package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_TPM_Response(t *testing.T) {
	mockAgent := NewMockAgent()
	ctx := context.Background()
	prompt := "You are an expert Technical Program Manager"

	resp, err := mockAgent.Send(ctx, prompt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(resp, "ID:[PRIMES]") {
		t.Errorf("expected response to contain 'ID:[PRIMES]', got: %s", resp)
	}

	if !strings.Contains(resp, `"title": "ID:[PRIMES] Implement prime number script"`) {
		t.Errorf("expected response to contain correct title with ID, got: %s", resp)
	}
}
