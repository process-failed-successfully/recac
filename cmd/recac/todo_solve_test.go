package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"recac/internal/agent"
)

func TestTodoSolveCmd(t *testing.T) {
	// 1. Setup Temp Dir
	tempDir, err := os.MkdirTemp("", "recac-test-todo-solve")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Switch CWD
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get cwd: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to chdir: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalWd); err != nil {
			t.Errorf("Failed to restore cwd: %v", err)
		}
	}()

	// 2. Setup Files
	targetFile := "main.go"
	// Note: scanTodos skips files if it thinks they are binary.
	// It reads first 512 bytes.
	// Also it ignores some dirs.
	// But in root, simple .go file is fine.

	initialContentReal := `package main

import "fmt"

func main() {
	// TODO: Implement greeting
}
`
	if err := os.WriteFile(targetFile, []byte(initialContentReal), 0644); err != nil {
		t.Fatalf("Failed to write target file: %v", err)
	}

	// 3. Mock Agent
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	mockAg := agent.NewMockAgent()
	expectedFixedContent := `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}
`
	// The agent returns the whole file content in a code block
	mockAg.SetResponse(fmt.Sprintf("```go\n%s\n```", expectedFixedContent))

	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAg, nil
	}

	// 4. Run Scan to populate TODO.md
	// We assume todoScanCmd is initialized in todo_scan.go init() -> todoCmd.AddCommand
	// But we can call the RunE function or the helper if exported.
	// scanAndAddTodos is unexported but we are in package main.
	// scanAndAddTodos(cmd, root)

	// We need a dummy command for logging
	todoScanCmd.SetOut(os.Stdout)
	todoScanCmd.SetErr(os.Stderr)

	if err := scanAndAddTodos(todoScanCmd, "."); err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Verify TODO.md content exists
	todoContent, err := os.ReadFile("TODO.md")
	if err != nil {
		t.Fatalf("Failed to read TODO.md: %v", err)
	}
	// Debug print
	t.Logf("TODO.md content:\n%s", string(todoContent))

	if !strings.Contains(string(todoContent), "TODO: Implement greeting") {
		t.Fatalf("TODO.md does not contain expected task. Content:\n%s", string(todoContent))
	}

	// 5. Run Solve
	// We want to solve task #1.
	todoSolveCmd.SetOut(os.Stdout)
	todoSolveCmd.SetErr(os.Stderr)

	// We execute runTodoSolve directly
	if err := runTodoSolve(todoSolveCmd, []string{"1"}); err != nil {
		t.Fatalf("Solve failed: %v", err)
	}

	// 6. Verify Results
	// Check file content
	finalContent, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("Failed to read target file: %v", err)
	}
	// utils.CleanCodeBlock trims spaces, so we should trim expected as well for comparison
	if strings.TrimSpace(string(finalContent)) != strings.TrimSpace(expectedFixedContent) {
		t.Errorf("File content mismatch.\nExpected:\n%q\nGot:\n%q", expectedFixedContent, string(finalContent))
	}

	// Check TODO.md
	finalTodo, err := os.ReadFile("TODO.md")
	if err != nil {
		t.Fatalf("Failed to read TODO.md: %v", err)
	}

	// The task should be marked as done: "- [x] ..."
	// scanAndAddTodos creates lines like "- [ ] [main.go:6] TODO: Implement greeting"
	// toggleTaskStatus changes "- [ ]" to "- [x]"

	expectedTodoLinePart := "[x] [main.go:6] TODO: Implement greeting"
	if !strings.Contains(string(finalTodo), expectedTodoLinePart) {
		// Try to see what is there
		t.Errorf("TODO.md was not updated correctly. Content:\n%s", string(finalTodo))
	}
}
