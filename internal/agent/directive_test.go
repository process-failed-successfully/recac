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

func TestDirectiveAgent_SendStream(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "directive_test_stream")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	recacDir := filepath.Join(tmpDir, ".recac")
	if err := os.Mkdir(recacDir, 0755); err != nil {
		t.Fatalf("Failed to create .recac dir: %v", err)
	}

	directiveFile := filepath.Join(recacDir, "directive")
	directiveContent := "Stream directive."
	if err := os.WriteFile(directiveFile, []byte(directiveContent), 0644); err != nil {
		t.Fatalf("Failed to write directive file: %v", err)
	}

	spy := &SpyAgent{}
	agent := NewDirectiveAgent(spy, tmpDir)

	originalPrompt := "Stream prompt."
	chunks := []string{}
	_, err = agent.SendStream(context.Background(), originalPrompt, func(s string) {
		chunks = append(chunks, s)
	})
	if err != nil {
		t.Fatalf("SendStream failed: %v", err)
	}

	if !strings.Contains(spy.lastPrompt, "[PROJECT DIRECTIVE]: "+directiveContent) {
		t.Errorf("Expected prompt to contain directive, got: %s", spy.lastPrompt)
	}
	if !strings.Contains(spy.lastPrompt, originalPrompt) {
		t.Errorf("Expected prompt to contain original prompt, got: %s", spy.lastPrompt)
	}
	if len(chunks) != 1 || chunks[0] != "mock response" {
		t.Errorf("Expected chunks to be [\"mock response\"], got %v", chunks)
	}
}

func TestDirectiveAgent_SendStream_NoDirective(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "directive_test_stream_empty")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	spy := &SpyAgent{}
	agent := NewDirectiveAgent(spy, tmpDir)

	originalPrompt := "Stream prompt."
	chunks := []string{}
	_, err = agent.SendStream(context.Background(), originalPrompt, func(s string) {
		chunks = append(chunks, s)
	})
	if err != nil {
		t.Fatalf("SendStream failed: %v", err)
	}

	if spy.lastPrompt != originalPrompt {
		t.Errorf("Expected prompt to be unmodified, got: %s", spy.lastPrompt)
	}
	if len(chunks) != 1 || chunks[0] != "mock response" {
		t.Errorf("Expected chunks to be [\"mock response\"], got %v", chunks)
	}
}

func TestDirectiveAgent_Send_Error(t *testing.T) {
	// Test error when directive is present but unreadable
	tmpDir, err := os.MkdirTemp("", "directive_test_error")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	recacDir := filepath.Join(tmpDir, ".recac")
	if err := os.Mkdir(recacDir, 0755); err != nil {
		t.Fatalf("Failed to create .recac dir: %v", err)
	}

	// Create directory with name 'directive' to force a read error
	directiveFile := filepath.Join(recacDir, "directive")
	if err := os.Mkdir(directiveFile, 0755); err != nil {
		t.Fatalf("Failed to create directive dir: %v", err)
	}

	spy := &SpyAgent{}
	agent := NewDirectiveAgent(spy, tmpDir)

	originalPrompt := "Test error prompt."
	_, err = agent.Send(context.Background(), originalPrompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// It should fallback to original prompt on read error
	if spy.lastPrompt != originalPrompt {
		t.Errorf("Expected prompt to fallback to original prompt, got: %s", spy.lastPrompt)
	}

	_, err = agent.SendStream(context.Background(), originalPrompt, nil)
	if err != nil {
		t.Fatalf("SendStream failed: %v", err)
	}

	if spy.lastPrompt != originalPrompt {
		t.Errorf("Expected prompt to fallback to original prompt for SendStream, got: %s", spy.lastPrompt)
	}
}

func TestDirectiveAgent_EmptyDirective(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "directive_test_empty_file")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	recacDir := filepath.Join(tmpDir, ".recac")
	if err := os.Mkdir(recacDir, 0755); err != nil {
		t.Fatalf("Failed to create .recac dir: %v", err)
	}

	directiveFile := filepath.Join(recacDir, "directive")
	// Create an empty directive file
	if err := os.WriteFile(directiveFile, []byte("   \n  "), 0644); err != nil {
		t.Fatalf("Failed to write directive file: %v", err)
	}

	spy := &SpyAgent{}
	agent := NewDirectiveAgent(spy, tmpDir)

	originalPrompt := "Prompt."
	_, err = agent.Send(context.Background(), originalPrompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Should not prepend anything if directive is empty
	if spy.lastPrompt != originalPrompt {
		t.Errorf("Expected prompt to be unmodified for empty directive, got: %s", spy.lastPrompt)
	}
}

func TestDirectiveAgent_Inner(t *testing.T) {
	spy := &SpyAgent{}
	agent := NewDirectiveAgent(spy, ".")
	if agent.Inner() != spy {
		t.Errorf("Inner agent mismatch")
	}
}
