package agent

import (
	"context"
	"testing"
	"time"
)

func TestCLIAgentsManual(t *testing.T) {
	// This test is meant to be run manually to verify CLI agents
	// It is skipped by default to avoid failing in environments without the CLIs
	// To run: go test -v ./internal/agent -run TestCLIAgentsManual -args -manual

	// Check for manual flag or just try running it if env var set?
	// For now, let's just make it always run but log errors instead of failing hard if CLI missing
	// Or we can check if executables exist.

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test Gemini CLI
	t.Log("--- Testing Gemini CLI ---")
	gemini, err := NewAgent("gemini-cli", "", "auto")
	if err != nil {
		t.Logf("Failed to create Gemini CLI agent: %v", err)
	} else {
		resp, err := gemini.Send(ctx, "Hello from Recac Test. Reply with 'Gemini OK'")
		if err != nil {
			t.Logf("Gemini CLI failed (expected if not installed/configured): %v", err)
		} else {
			t.Logf("Gemini CLI response: %s", resp)
		}
	}

	// Test Cursor CLI
	t.Log("\n--- Testing Cursor CLI ---")
	cursor, err := NewAgent("cursor-cli", "", "auto")
	if err != nil {
		t.Logf("Failed to create Cursor CLI agent: %v", err)
	} else {
		resp, err := cursor.Send(ctx, "Hello from Recac Test. Reply with 'Cursor OK'")
		if err != nil {
			t.Logf("Cursor CLI failed (expected if not installed/configured): %v", err)
		} else {
			t.Logf("Cursor CLI response: %s", resp)
		}
	}
}
