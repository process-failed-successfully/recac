package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"testing"

	"github.com/AlecAivazis/survey/v2"
	"github.com/stretchr/testify/assert"
)

func TestQuizCmd(t *testing.T) {
	// 1. Setup temporary directory with a Go file
	tmpDir, err := os.MkdirTemp("", "recac-quiz-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.go")
	content := `package main
func Add(a, b int) int { return a + b }`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// 2. Mock Agent Factory
	originalFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		mock := agent.NewMockAgent()
		// Return a valid JSON response
		mock.SetResponse(`{
			"question": "What does this function do?",
			"options": ["Adds two numbers", "Subtracts two numbers", "Multiplies two numbers"],
			"correct_answer": "Adds two numbers",
			"explanation": "It uses the + operator."
		}`)
		return mock, nil
	}
	defer func() { agentClientFactory = originalFactory }()

	// 3. Mock Survey (askOneFunc)
	originalAskOne := askOneFunc
	// We want to simulate the user selecting the correct answer
	askOneFunc = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
		// Verify we got the expected prompt type
		selectPrompt, ok := p.(*survey.Select)
		if !ok {
			return fmt.Errorf("expected Select prompt")
		}
		if selectPrompt.Message != "What does this function do?" {
			return fmt.Errorf("unexpected question: %s", selectPrompt.Message)
		}

		// Set the response (pointer to string)
		respStr, ok := response.(*string)
		if !ok {
			return fmt.Errorf("response expected to be *string")
		}
		*respStr = "Adds two numbers"
		return nil
	}
	defer func() { askOneFunc = originalAskOne }()

	// 4. Run Command
	// We need to set the args to point to our tmpDir
	cmd := quizCmd
	cmd.SetArgs([]string{tmpDir})

	// Capture output
	// Note: We are not using cmd.SetOut here because we want to see if it runs without error first.
	// But if we want to assert on output, we should use a buffer.
	// However, cobra's Execute/ExecuteC captures output if set on root, but we are running RunE directly?
	// No, we should call cmd.Execute() or runQuiz directly.
	// Calling runQuiz directly is easier for unit testing internal logic, but we need to set the context/flags if needed.
	// quizCmd doesn't have flags yet.

	// But wait, runQuiz uses cmd.OutOrStdout().
	// We can set it.
	// buffer := new(bytes.Buffer)
	// cmd.SetOut(buffer)
	// But `askOneFunc` mock doesn't write to cmd.Out, it just sets the value.
	// The `runQuiz` prints "Correct!".

	// Let's just run it and check for error for now.
	err = runQuiz(cmd, []string{tmpDir})
	assert.NoError(t, err)

	// 5. Test Incorrect Answer
	askOneFunc = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
		respStr, _ := response.(*string)
		*respStr = "Subtracts two numbers" // Wrong answer
		return nil
	}

	err = runQuiz(cmd, []string{tmpDir})
	assert.NoError(t, err)
}

func TestGetRandomGoFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "recac-quiz-file-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Case 1: No files
	_, _, err = getRandomGoFile(tmpDir)
	assert.Error(t, err)

	// Case 2: Only test files
	os.WriteFile(filepath.Join(tmpDir, "foo_test.go"), []byte("package foo"), 0644)
	_, _, err = getRandomGoFile(tmpDir)
	assert.Error(t, err)

	// Case 3: Valid file
	os.WriteFile(filepath.Join(tmpDir, "foo.go"), []byte("package foo"), 0644)
	content, path, err := getRandomGoFile(tmpDir)
	assert.NoError(t, err)
	assert.Contains(t, content, "package foo")
	assert.Equal(t, "foo.go", path)
}
