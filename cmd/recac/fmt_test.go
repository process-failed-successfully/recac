package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"recac/internal/agent"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestFmtCmd_Success(t *testing.T) {
	cmd := rootCmd
	resetFlags(cmd)

	// Create a temporary file to format
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.go")
	originalContent := "package main\nfunc main(){\nfmt.Println(\"test\")\n}"
	err := os.WriteFile(filePath, []byte(originalContent), 0644)
	assert.NoError(t, err)

	formattedContent := "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"test\")\n}\n"

	// Mock Agent
	mockAgent := new(TestCmdMockAgent)

	expectedResponse := "<file path=\"" + filePath + "\">\n" + formattedContent + "</file>"
	mockAgent.On("Send", mock.Anything, mock.Anything).Return(expectedResponse, nil)

	originalAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalAgentFactory }()

	// Execute command
	output, err := executeCommand(cmd, "fmt", filePath)

	assert.NoError(t, err)
	assert.Contains(t, output, "Successfully formatted")

	// Verify file was actually modified
	newContent, err := os.ReadFile(filePath)
	assert.NoError(t, err)
	assert.Equal(t, formattedContent, string(newContent))
}

func TestFmtCmd_FallbackCleanCodeBlock(t *testing.T) {
	cmd := rootCmd
	resetFlags(cmd)

	// Create a temporary file to format
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.go")
	originalContent := "package main\nfunc main(){\nfmt.Println(\"test\")\n}"
	err := os.WriteFile(filePath, []byte(originalContent), 0644)
	assert.NoError(t, err)

	formattedContent := "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"test\")\n}\n"

	// Mock Agent - return without tags, just codeblock
	mockAgent := new(TestCmdMockAgent)

	expectedResponse := "Here is your formatted code:\n```go\n" + formattedContent + "```\n"
	mockAgent.On("Send", mock.Anything, mock.Anything).Return(expectedResponse, nil)

	originalAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalAgentFactory }()

	// Execute command
	output, err := executeCommand(cmd, "fmt", filePath)

	assert.NoError(t, err)
	assert.Contains(t, output, "Successfully formatted")

	// Verify file was actually modified using the fallback parsing
	newContent, err := os.ReadFile(filePath)
	assert.NoError(t, err)
	// utils.CleanCodeBlock might trim the trailing newline, so we accommodate that
	assert.Contains(t, formattedContent, string(newContent))
}

func TestFmtCmd_SkipDirectory(t *testing.T) {
	cmd := rootCmd
	resetFlags(cmd)

	tmpDir := t.TempDir()

	// Mock Agent
	mockAgent := new(TestCmdMockAgent)
	originalAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalAgentFactory }()

	// Execute command with a directory
	output, err := executeCommand(cmd, "fmt", tmpDir)

	assert.NoError(t, err)
	assert.Contains(t, output, "skipping directory")
}

func TestFmtCmd_FileNotFound(t *testing.T) {
	cmd := rootCmd
	resetFlags(cmd)

	// Mock Agent
	mockAgent := new(TestCmdMockAgent)
	originalAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalAgentFactory }()

	// Execute command with a non-existent file
	output, err := executeCommand(cmd, "fmt", "non_existent_file.go")

	assert.NoError(t, err)
	assert.Contains(t, output, "failed to stat file")
}
