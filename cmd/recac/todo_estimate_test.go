package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"recac/internal/agent"
)

// MockTodoEstimateAgent for testing
type MockTodoEstimateAgent struct {
	mock.Mock
}

func (m *MockTodoEstimateAgent) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *MockTodoEstimateAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	args := m.Called(ctx, prompt, onChunk)
	return args.String(0), args.Error(1)
}

func TestTodoEstimateCmd(t *testing.T) {
	// Setup temporary directory
	tempDir, err := os.MkdirTemp("", "recac-todo-estimate-test")
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
	// TODO: Refactor this mess
}
`
	err = os.WriteFile(sourceFile, []byte(originalCode), 0644)
	assert.NoError(t, err)

	// Create TODO.md with a valid task
	taskEntry := fmt.Sprintf("- [ ] [%s:4] TODO: Refactor this mess", sourceFile)
	err = os.WriteFile("TODO.md", []byte("# TODO\n\n"+taskEntry+"\n"), 0644)
	assert.NoError(t, err)

	// Mock the agent factory
	origFactory := agentClientFactory
	defer func() { agentClientFactory = origFactory }()

	mockAgent := new(MockTodoEstimateAgent)
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	jsonResponse := `{
  "summary": "Refactor into separate functions",
  "complexity": "Medium",
  "story_points": 3,
  "estimated_hours": "2-4h",
  "risks": ["Regression"],
  "implementation_steps": ["Step 1"]
}`

	mockAgent.On("Send", mock.Anything, mock.Anything).Return(jsonResponse, nil).Once()

	// Execute command
	cmd := todoEstimateCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(new(bytes.Buffer))

	err = runTodoEstimate(cmd, 1)
	assert.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "ESTIMATION REPORT")
	assert.Contains(t, output, "Refactor into separate functions")
	assert.Contains(t, output, "Story Points")
	assert.Contains(t, output, "3")
}

func TestTodoEstimateCmd_InvalidIndex(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "recac-todo-estimate-fail")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	oldWd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(oldWd)

	err = os.WriteFile("TODO.md", []byte("# TODO\n\n- [ ] Task 1\n"), 0644)
	assert.NoError(t, err)

	cmd := todoEstimateCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err = runTodoEstimate(cmd, 99)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "index 99 not found")
}

func TestTodoEstimateCmd_NoFileLocation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "recac-todo-estimate-nofile")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	oldWd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(oldWd)

	// Task without file info
	err = os.WriteFile("TODO.md", []byte("# TODO\n\n- [ ] Simple task\n"), 0644)
	assert.NoError(t, err)

	cmd := todoEstimateCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err = runTodoEstimate(cmd, 1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "could not identify file")
}
