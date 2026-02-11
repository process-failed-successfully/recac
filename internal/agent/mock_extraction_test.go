package agent

import (
	"context"
	"strings"
	"testing"
)

func TestExtractTicketID(t *testing.T) {
	tests := []struct {
		prompt   string
		expected string
	}{
		{"Review ticket MFLP-12345", "MFLP-12345"},
		{"Work on MFLP-101", "MFLP-101"},
		{"No ticket here", ""},
		{"Repo: https://github.com/process-failed-successfully/recac-jira-e2e (MFLP-999)", "MFLP-999"},
	}

	for _, tt := range tests {
		got := extractTicketID(tt.prompt)
		if got != tt.expected {
			t.Errorf("extractTicketID(%q) = %q, want %q", tt.prompt, got, tt.expected)
		}
	}
}

func TestMockAgent_DynamicBranch(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// Test Initializer with Ticket ID (Primes scenario)
	promptInit := "ROLE - INITIALIZER AGENT\nRepo: ... (MFLP-500)\nTask: Implement prime number generator."
	respInit, err := agent.Send(ctx, promptInit)
	if err != nil {
		t.Fatalf("Init Send failed: %v", err)
	}
	if !strings.Contains(respInit, "\"id\": \"MFLP-500\"") {
		t.Errorf("Initializer response should contain dynamic ID MFLP-500, got: %s", respInit)
	}

	// Test TPM with Ticket ID
	promptTPM := "Technical Program Manager\nTicket: MFLP-555"
	respTPM, err := agent.Send(ctx, promptTPM)
	if err != nil {
		t.Fatalf("TPM Send failed: %v", err)
	}
	if !strings.Contains(respTPM, "\"id\": \"MFLP-555\"") {
		t.Errorf("TPM response should contain dynamic ID MFLP-555, got: %s", respTPM)
	}

	// Test Coding Agent with Ticket ID in header
	promptCodingHeader := "**Feature ID**: MFLP-600\nPlease implement primes.py"
	respCoding, err := agent.Send(ctx, promptCodingHeader)
	if err != nil {
		t.Fatalf("Coding Send failed: %v", err)
	}
	if !strings.Contains(respCoding, "git checkout -B agent/MFLP-600") {
		t.Errorf("Coding response should checkout dynamic branch agent/MFLP-600, got: %s", respCoding)
	}
	if !strings.Contains(respCoding, "agent-bridge feature set MFLP-600") {
		t.Errorf("Coding response should update feature MFLP-600, got: %s", respCoding)
	}

	// Test Coding Agent fallback (no header, but ticket ID in text)
	promptCodingFallback := "Implement primes.py for ticket MFLP-700"
	respCodingFallback, err := agent.Send(ctx, promptCodingFallback)
	if err != nil {
		t.Fatalf("Coding Fallback Send failed: %v", err)
	}
	if !strings.Contains(respCodingFallback, "git checkout -B agent/MFLP-700") {
		t.Errorf("Coding response should fallback to dynamic branch agent/MFLP-700, got: %s", respCodingFallback)
	}

	// Test Default behavior (PRIMES)
	promptDefault := "Implement primes.py"
	respDefault, err := agent.Send(ctx, promptDefault)
	if err != nil {
		t.Fatalf("Default Send failed: %v", err)
	}
	if !strings.Contains(respDefault, "git checkout -B agent/PRIMES-mock") {
		t.Errorf("Default response should use PRIMES-mock, got: %s", respDefault)
	}
}
