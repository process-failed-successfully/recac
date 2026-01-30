package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

// MockAgentForTranslate is a mock implementation of agent.Agent
type MockAgentForTranslate struct {
	Response string
}

func (m *MockAgentForTranslate) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, nil
}

func (m *MockAgentForTranslate) SendStream(ctx context.Context, prompt string, callback func(string)) (string, error) {
	callback(m.Response)
	return m.Response, nil
}

func TestTranslateCmd_SingleFile(t *testing.T) {
	// Setup temporary directory
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "hello.go")
	outputFile := filepath.Join(tmpDir, "hello.py")

	// Create a dummy Go file
	sourceContent := `package main
import "fmt"
func main() { fmt.Println("Hello") }`
	err := os.WriteFile(sourceFile, []byte(sourceContent), 0644)
	if err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// Mock the agent factory
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	expectedTranslatedCode := `def main():
    print("Hello")

if __name__ == "__main__":
    main()`

	// The command expects a code block from the agent
	mockResponse := "Here is the code:\n```python\n" + expectedTranslatedCode + "\n```"

	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return &MockAgentForTranslate{Response: mockResponse}, nil
	}

	// Set flags via variables since the command binds to them
	translateTarget = "python"
	translateOutput = outputFile
	translateStdout = false
	translatePrompt = ""

	// We need to set viper values as well since factories use them
	viper.Set("provider", "mock")
	viper.Set("model", "mock-model")

	// Execute runTranslate
	// We need to reset the command output to stdout just in case
	translateCmd.SetOut(os.Stdout)

	err = runTranslate(translateCmd, []string{sourceFile})
	assert.NoError(t, err)

	// Verify output
	content, err := os.ReadFile(outputFile)
	assert.NoError(t, err)
	assert.Equal(t, expectedTranslatedCode, string(content))
}

func TestTranslateCmd_Stdout_Capture(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "hello.go")
	os.WriteFile(sourceFile, []byte("package main"), 0644)

	// Mock agent
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	mockResponse := "print('Hello')"
	agentClientFactory = func(ctx context.Context, p, m, pp, pn string) (agent.Agent, error) {
		return &MockAgentForTranslate{Response: mockResponse}, nil
	}

	// Set flags
	translateTarget = "python"
	translateOutput = ""
	translateStdout = true
	translatePrompt = ""

	viper.Set("provider", "mock")
	viper.Set("model", "mock-model")

	// Use a buffer for output
	buf := new(bytes.Buffer)
	translateCmd.SetOut(buf)

	err := runTranslate(translateCmd, []string{sourceFile})
	assert.NoError(t, err)

	assert.Contains(t, buf.String(), "print('Hello')")
}
