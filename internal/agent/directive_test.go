package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// SpyAgent records the prompt it receives
type SpyAgent struct {
	lastPrompt string
}

func (s *SpyAgent) Send(ctx context.Context, prompt string) (string, error) {
	s.lastPrompt = prompt
	return "mock response", nil
}

func (s *SpyAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	s.lastPrompt = prompt
	if onChunk != nil {
		onChunk("mock response")
	}
	return "mock response", nil
}

func TestDirectiveAgent_Send(t *testing.T) {
	// Create a temporary directory for tests
	tmpDir, err := os.MkdirTemp("", "directive_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create .recac directory
	recacDir := filepath.Join(tmpDir, ".recac")
	if err := os.Mkdir(recacDir, 0755); err != nil {
		t.Fatalf("Failed to create .recac dir: %v", err)
	}

	directiveFile := filepath.Join(recacDir, "directive")
	directiveContent := "Always use Go best practices."
	if err := os.WriteFile(directiveFile, []byte(directiveContent), 0644); err != nil {
		t.Fatalf("Failed to write directive file: %v", err)
	}

	// Create spy agent
	spy := &SpyAgent{}

	// Create DirectiveAgent
	agent := NewDirectiveAgent(spy, tmpDir)

	// Call Send
	originalPrompt := "Write a hello world function."
	_, err = agent.Send(context.Background(), originalPrompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Verify prompt was modified
	if !strings.Contains(spy.lastPrompt, "[PROJECT DIRECTIVE]: "+directiveContent) {
		t.Errorf("Expected prompt to contain directive, got: %s", spy.lastPrompt)
	}
	if !strings.Contains(spy.lastPrompt, originalPrompt) {
		t.Errorf("Expected prompt to contain original prompt, got: %s", spy.lastPrompt)
	}
}

func TestDirectiveAgent_NoDirective(t *testing.T) {
	// Create a temporary directory for tests
	tmpDir, err := os.MkdirTemp("", "directive_test_empty")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Do NOT create directive file

	// Create spy agent
	spy := &SpyAgent{}

	// Create DirectiveAgent
	agent := NewDirectiveAgent(spy, tmpDir)

	// Call Send
	originalPrompt := "Write a hello world function."
	_, err = agent.Send(context.Background(), originalPrompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Verify prompt was NOT modified
	if spy.lastPrompt != originalPrompt {
		t.Errorf("Expected prompt to be unmodified, got: %s", spy.lastPrompt)
	}
}
