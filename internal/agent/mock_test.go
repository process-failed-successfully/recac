package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Send_TPM(t *testing.T) {
	agent := NewMockAgent()
	prompt := "You are an expert Technical Program Manager (TPM)..."
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(response, "\"type\": \"Epic\"") {
		t.Errorf("expected JSON response, got: %s", response)
	}
}

func TestMockAgent_Send_QA(t *testing.T) {
	agent := NewMockAgent()
	prompt := "You are a QA Engineer..."
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if response != "QA_PASSED" {
		t.Errorf("expected QA_PASSED, got: %s", response)
	}
}

func TestMockAgent_Send_Manager(t *testing.T) {
	agent := NewMockAgent()
	prompt := "You are an Engineering Manager... sign off"
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if response != "PROJECT_SIGNED_OFF" {
		t.Errorf("expected PROJECT_SIGNED_OFF, got: %s", response)
	}
}

func TestMockAgent_Send_Default(t *testing.T) {
	agent := NewMockAgent()
	prompt := "Write some python code"
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(response, "Mock agent response") {
		t.Errorf("expected default mock response, got: %s", response)
	}
}
