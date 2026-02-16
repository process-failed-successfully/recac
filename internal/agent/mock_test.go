package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Send_TPM(t *testing.T) {
	agent := NewMockAgent()
	prompt := "You are an expert Technical Program Manager. Please create a plan for ID:[PRIMES]."

	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(resp, "ID:[PRIMES]") {
		t.Errorf("Expected response to contain ID:[PRIMES], got: %s", resp)
	}
	if !strings.Contains(resp, "[") || !strings.Contains(resp, "{") {
		t.Errorf("Expected response to look like JSON, got: %s", resp)
	}
}

func TestMockAgent_Send_Coding(t *testing.T) {
	agent := NewMockAgent()
	prompt := "## YOUR ROLE - CODING AGENT\nImplement prime number generator"

	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(resp, "```bash") {
		t.Errorf("Expected response to contain bash block, got: %s", resp)
	}
	if !strings.Contains(resp, "primes.py") {
		t.Errorf("Expected response to create primes.py, got: %s", resp)
	}
}
