package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"recac/internal/agent"

	tea "github.com/charmbracelet/bubbletea"
)

// MockReviewAgent implements agent.Agent for testing
type MockReviewAgent struct {
	ResponseJSON string
}

func (m *MockReviewAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.ResponseJSON, nil
}

func (m *MockReviewAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return m.ResponseJSON, nil
}

func TestReviewCmd_Interactive(t *testing.T) {
	// Setup
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "buggy.go")
	content := "package main\nfunc main() { panic(1) }"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Prepare mock agent response
	issues := []ReviewIssue{
		{
			File:        filePath,
			Line:        2,
			Title:       "Panic used",
			Description: "Avoid using panic in main.",
			Severity:    "CRITICAL",
			Suggestion:  "log.Fatal(1)",
		},
	}
	jsonBytes, _ := json.Marshal(issues)
	mockAgent := &MockReviewAgent{ResponseJSON: string(jsonBytes)}

	// Override factories
	originalAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalAgentFactory }()

	// Override TUI runner
	originalTUIFunc := runReviewTUIFunc
	var capturedModel tea.Model
	runReviewTUIFunc = func(m tea.Model) error {
		capturedModel = m
		return nil
	}
	defer func() { runReviewTUIFunc = originalTUIFunc }()

	// Override Interactive flag
	// Since reviewInteractive is a global variable bound to the flag, we can set it directly?
	// Butcobra flags bind to pointers.
	// We can just set the flag via args if we use Execute, or set the variable if we use RunE directly.
	// Better to use a new command instance and set the flag via SetArgs or directly.

	// Create command
	cmd := NewReviewCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// We can manually set the flag value on the command execution
	// parsing args will set reviewInteractive
	cmd.SetArgs([]string{"--interactive", filePath})

	// Execute
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify TUI was called
	if capturedModel == nil {
		t.Fatal("TUI was not started")
	}

	// Verify model content
	reviewModel, ok := capturedModel.(ReviewModel)
	if !ok {
		t.Fatal("Captured model is not of type ReviewModel")
	}

	if len(reviewModel.issues) != 1 {
		t.Errorf("Expected 1 issue, got %d", len(reviewModel.issues))
	}
	if reviewModel.issues[0].Title != "Panic used" {
		t.Errorf("Expected title 'Panic used', got '%s'", reviewModel.issues[0].Title)
	}
}

func TestApplyFixCmd(t *testing.T) {
	// Setup
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "fixme.go")
	content := "line 1\nline 2\nline 3\nline 4"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	issue := &ReviewIssue{
		File:       filePath,
		Line:       3, // "line 3"
		Title:      "Fix this",
		Suggestion: "Better line 3",
	}

	// Execute
	cmd := applyFixCmd(issue)
	msg := cmd() // Run the command function

	// Verify Msg
	if _, ok := msg.(fixMsg); !ok {
		if errMsg, ok := msg.(errMsg); ok {
			t.Fatalf("Command returned error: %v", errMsg.err)
		}
		t.Fatalf("Command returned unexpected message: %T", msg)
	}

	// Verify File Content
	newContentBytes, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	newContent := string(newContentBytes)

	expected := "line 1\nline 2\n// TODO(recac-fix): Fix this\n// Suggestion:\n// Better line 3\nline 3\nline 4"
	if newContent != expected {
		t.Errorf("File content mismatch.\nExpected:\n%s\nGot:\n%s", expected, newContent)
	}
}

// TestApplyFixCmd_OutOfBounds verifies error handling for invalid lines
func TestApplyFixCmd_OutOfBounds(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "short.go")
	os.WriteFile(filePath, []byte("line 1"), 0644)

	issue := &ReviewIssue{
		File:       filePath,
		Line:       10,
		Title:      "Bad line",
		Suggestion: "stuff",
	}

	cmd := applyFixCmd(issue)
	msg := cmd()

	if _, ok := msg.(errMsg); !ok {
		t.Errorf("Expected errMsg, got %T", msg)
	}
}
