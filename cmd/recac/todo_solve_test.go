package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"recac/internal/agent"
)

// MockSolveAgent for testing
type MockSolveAgent struct {
	mock.Mock
}

func (m *MockSolveAgent) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *MockSolveAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	args := m.Called(ctx, prompt, onChunk)
	return args.String(0), args.Error(1)
}

func TestTodoSolveCmd(t *testing.T) {
	// Setup temporary directory
	tempDir, err := os.MkdirTemp("", "recac-todo-solve-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Change cwd to tempDir so TODO.md is created there
	oldWd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(oldWd)

	// Create a dummy source file
	sourceFile := "main.go"
	originalCode := `package main

func main() {
	// TODO: Print hello
}
`
	err = os.WriteFile(sourceFile, []byte(originalCode), 0644)
	assert.NoError(t, err)

	// Create TODO.md with a valid task
	// task format from todo scan: [File:Line] Keyword: Content
	taskEntry := fmt.Sprintf("- [ ] [%s:4] TODO: Print hello", sourceFile)
	err = os.WriteFile("TODO.md", []byte("# TODO\n\n"+taskEntry+"\n"), 0644)
	assert.NoError(t, err)

	// Mock the agent factory
	origFactory := agentClientFactory
	defer func() { agentClientFactory = origFactory }()

	mockAgent := new(MockSolveAgent)
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	improvedCode := `package main

import "fmt"

func main() {
	fmt.Println("hello")
}
`
	// The agent returns the code, and our command uses utils.CleanCodeBlock which trims it.
	// So we expect the trimmed version in the file.
	expectedContent := strings.TrimSpace(improvedCode)
	mockAgent.On("Send", mock.Anything, mock.Anything).Return(improvedCode, nil).Once()

	// Execute command
	cmd := todoSolveCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(new(bytes.Buffer))

	// Run todo solve 1
	// Note: We need to pass args via SetArgs if we were executing root, but since we exec subcmd directly:
	// But cobra subcommands expect args passed to RunE. However, using ExecuteC or similar is better if attached to root.
	// Here we can just call RunE manually or set args.
	// But `todoSolveCmd` is attached to `todoCmd` which is attached to `rootCmd`.
	// For unit test of just this command, we can just invoke RunE logic via a wrapper or assume SetArgs works if we Exec the command itself?
	// cobra.Command.Execute() executes the command. If we set args, it parses.
	// BUT `todoSolveCmd` is not the root.
	// Let's try calling runTodoSolve directly or use a fresh command structure if needed.
	// Actually, `cmd.Execute` works if it's the root of execution for the test.
	// But `todoSolveCmd` expects 1 arg.

	// Let's use `todoSolveCmd.RunE` directly for simplicity, mocking the context/args.
	err = runTodoSolve(cmd, 1)
	assert.NoError(t, err)

	// Verify file updated
	content, err := os.ReadFile(sourceFile)
	assert.NoError(t, err)
	assert.Equal(t, expectedContent, string(content))

	// Verify TODO.md updated
	todoContent, err := os.ReadFile("TODO.md")
	assert.NoError(t, err)
	assert.Contains(t, string(todoContent), "- [x] [main.go:4] TODO: Print hello")
}

func TestTodoSolveCmd_InvalidIndex(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "recac-todo-solve-fail")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	oldWd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(oldWd)

	err = os.WriteFile("TODO.md", []byte("# TODO\n\n- [ ] Task 1\n"), 0644)
	assert.NoError(t, err)

	cmd := todoSolveCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err = runTodoSolve(cmd, 99)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "index 99 not found")
}

func TestTodoSolveCmd_NoFileLocation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "recac-todo-solve-nofile")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	oldWd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(oldWd)

	// Task without file info
	err = os.WriteFile("TODO.md", []byte("# TODO\n\n- [ ] Simple task\n"), 0644)
	assert.NoError(t, err)

	cmd := todoSolveCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err = runTodoSolve(cmd, 1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "could not identify file")
}
