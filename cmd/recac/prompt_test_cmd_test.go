package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"testing"

	"github.com/stretchr/testify/assert"
)

// MockAgentCaptor captures the prompt for verification
type MockAgentCaptor struct {
	*agent.MockAgent
	CapturedPrompt string
}

func (m *MockAgentCaptor) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	m.CapturedPrompt = prompt
	return m.MockAgent.SendStream(ctx, prompt, onChunk)
}

func TestPromptTestCmd_DryRun(t *testing.T) {
	// Setup Temp Dir for .recac/prompts
	cwd, _ := os.Getwd()
	tempDir := t.TempDir()
	os.Chdir(tempDir)
	defer os.Chdir(cwd)

	promptsDir := filepath.Join(tempDir, ".recac", "prompts")
	err := os.MkdirAll(promptsDir, 0755)
	assert.NoError(t, err)

	// Create test prompt
	promptPath := filepath.Join(promptsDir, "test_dry_run.md")
	err = os.WriteFile(promptPath, []byte("Hello {name}!"), 0644)
	assert.NoError(t, err)

	// Run command
	buf := new(bytes.Buffer)
	promptTestCmd.SetOut(buf)
	promptTestCmd.SetErr(buf)

	// Reset flags manually because they are package global variables
	ptVars = []string{}
	ptJsonFile = ""
	ptDryRun = false
	ptModel = ""
	ptSaveFile = ""

	// Use rootCmd to ensure proper command resolution
	rootCmd.SetArgs([]string{"prompt", "test", "test_dry_run", "--var", "name=World", "--dry-run"})
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)

	err = rootCmd.Execute()
	assert.NoError(t, err)

	assert.Contains(t, buf.String(), "Hello World!")
}

func TestPromptTestCmd_JsonFile(t *testing.T) {
	// Setup Temp Dir
	cwd, _ := os.Getwd()
	tempDir := t.TempDir()
	os.Chdir(tempDir)
	defer os.Chdir(cwd)

	promptsDir := filepath.Join(tempDir, ".recac", "prompts")
	os.MkdirAll(promptsDir, 0755)

	// Create prompt
	os.WriteFile(filepath.Join(promptsDir, "test_json.md"), []byte("Hello {name}! Age: {age}"), 0644)

	// Create JSON file
	jsonPath := filepath.Join(tempDir, "vars.json")
	os.WriteFile(jsonPath, []byte(`{"name": "JSON", "age": "42"}`), 0644)

	// Run command
	buf := new(bytes.Buffer)
	promptTestCmd.SetOut(buf)
	promptTestCmd.SetErr(buf)

	// Reset flags
	ptVars = []string{}
	ptJsonFile = ""
	ptDryRun = false
	ptModel = ""
	ptSaveFile = ""

	rootCmd.SetArgs([]string{"prompt", "test", "test_json", "--json-file", jsonPath, "--dry-run"})
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)

	err := rootCmd.Execute()
	assert.NoError(t, err)

	assert.Contains(t, buf.String(), "Hello JSON! Age: 42")
}

func TestPromptTestCmd_LiveAgent(t *testing.T) {
	// Setup Temp Dir
	cwd, _ := os.Getwd()
	tempDir := t.TempDir()
	os.Chdir(tempDir)
	defer os.Chdir(cwd)

	promptsDir := filepath.Join(tempDir, ".recac", "prompts")
	os.MkdirAll(promptsDir, 0755)

	// Create prompt
	os.WriteFile(filepath.Join(promptsDir, "test_live.md"), []byte("Hello {name}!"), 0644)

	// Setup Mock Agent
	mockAgent := &MockAgentCaptor{MockAgent: agent.NewMockAgent()}
	mockAgent.SetResponse("I am AI")

	// Override factory
	oldFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = oldFactory }()

	// Run command
	buf := new(bytes.Buffer)
	promptTestCmd.SetOut(buf)
	promptTestCmd.SetErr(buf)

	// Reset flags
	ptVars = []string{}
	ptJsonFile = ""
	ptDryRun = false
	ptModel = ""
	ptSaveFile = ""

	rootCmd.SetArgs([]string{"prompt", "test", "test_live", "--var", "name=AI"})
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)

	err := rootCmd.Execute()
	assert.NoError(t, err)

	// Verify Output
	assert.Contains(t, buf.String(), "I am AI")

	// Verify Prompt Sent
	assert.Equal(t, "Hello AI!", mockAgent.CapturedPrompt)
}

func TestPromptTestCmd_SaveFile(t *testing.T) {
	// Setup Temp Dir
	cwd, _ := os.Getwd()
	tempDir := t.TempDir()
	os.Chdir(tempDir)
	defer os.Chdir(cwd)

	promptsDir := filepath.Join(tempDir, ".recac", "prompts")
	os.MkdirAll(promptsDir, 0755)

	// Create prompt
	os.WriteFile(filepath.Join(promptsDir, "test_save.md"), []byte("Test"), 0644)

	// Setup Mock Agent
	mockAgent := agent.NewMockAgent()
	mockAgent.SetResponse("Saved Response")

	// Override factory
	oldFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = oldFactory }()

	// Save file path
	savePath := filepath.Join(tempDir, "response.txt")

	// Run command
	buf := new(bytes.Buffer)
	promptTestCmd.SetOut(buf)
	promptTestCmd.SetErr(buf)

	// Reset flags
	ptVars = []string{}
	ptJsonFile = ""
	ptDryRun = false
	ptModel = ""
	ptSaveFile = ""

	rootCmd.SetArgs([]string{"prompt", "test", "test_save", "--save", savePath})
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)

	err := rootCmd.Execute()
	assert.NoError(t, err)

	// Verify File Created
	content, err := os.ReadFile(savePath)
	assert.NoError(t, err)
	assert.Equal(t, "Saved Response", string(content))
}
