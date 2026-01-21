package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"recac/internal/agent"

	"github.com/AlecAivazis/survey/v2"
	"github.com/stretchr/testify/assert"
)

// MockAgent implementation for testing
type ResolveSpyAgent struct {
	SendFunc    func(ctx context.Context, prompt string) (string, error)
}

func (m *ResolveSpyAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.SendFunc != nil {
		return m.SendFunc(ctx, prompt)
	}
	return "", nil
}

func (m *ResolveSpyAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	resp, err := m.Send(ctx, prompt)
	if err == nil && onChunk != nil {
		onChunk(resp)
	}
	return resp, err
}

func TestResolveCmd_Manual_SingleFile(t *testing.T) {
	// Setup
	originalWd, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	// Create a conflicted file
	conflictedContent := `
package main

func main() {
<<<<<<< HEAD
	fmt.Println("Hello World")
=======
	fmt.Println("Goodbye World")
>>>>>>> feature
}
`
	conflictedFile := filepath.Join(tmpDir, "main.go")
	err := os.WriteFile(conflictedFile, []byte(conflictedContent), 0644)
	assert.NoError(t, err)

	// Mock Agent
	mockAgent := &ResolveSpyAgent{
		SendFunc: func(ctx context.Context, prompt string) (string, error) {
			// Verify prompt contains the conflict
			if !strings.Contains(prompt, "Hello World") || !strings.Contains(prompt, "Goodbye World") {
				return "", fmt.Errorf("prompt missing conflict context")
			}
			return "func main() {\n\tfmt.Println(\"Hello World\")\n}", nil
		},
	}

	// Mock Factories
	originalAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalAgentFactory }()

	// Mock AskOne (Interactive)
	originalAskOne := askOneFunc
	askOneFunc = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
		// Simulate "Accept"
		if s, ok := p.(*survey.Select); ok {
			if strings.Contains(s.Message, "Accept resolution") {
				*(response.(*string)) = "Accept"
				return nil
			}
		}
		// Simulate "No" for git add to avoid git command execution
		if c, ok := p.(*survey.Confirm); ok {
			if strings.Contains(c.Message, "git add") {
				*(response.(*bool)) = false
				return nil
			}
		}
		return nil
	}
	defer func() { askOneFunc = originalAskOne }()

	// Execute with --file
	cmd := resolveCmd
	// Reset flags
	resolveFile = ""
	resolveAuto = false

	// We use Parse to simulate CLI args parsing into globals
	cmd.Flags().Parse([]string{"--file", conflictedFile})

	err = cmd.RunE(cmd, []string{})
	assert.NoError(t, err)

	// Verify file content
	content, err := os.ReadFile(conflictedFile)
	assert.NoError(t, err)
	assert.Equal(t, "func main() {\n\tfmt.Println(\"Hello World\")\n}", string(content))
}

func TestResolveCmd_Auto_SingleFile(t *testing.T) {
	// Setup
	originalWd, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	conflictedFile := filepath.Join(tmpDir, "auto.go")
	os.WriteFile(conflictedFile, []byte("<<<<<<< HEAD\nA\n=======\nB\n>>>>>>> feature\n"), 0644)

	// Mock Agent
	mockAgent := &ResolveSpyAgent{
		SendFunc: func(ctx context.Context, prompt string) (string, error) {
			return "RESOLVED", nil
		},
	}
	originalAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalAgentFactory }()

	// Execute with --file and --auto
	cmd := resolveCmd
	resolveFile = ""
	resolveAuto = false
	cmd.Flags().Parse([]string{"--file", conflictedFile, "--auto"})

	err := cmd.RunE(cmd, []string{})
	assert.NoError(t, err)

	content, _ := os.ReadFile(conflictedFile)
	assert.Equal(t, "RESOLVED", string(content))
}

func TestResolveCmd_AutoDetect(t *testing.T) {
	// Setup
	originalWd, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	conflictedFile := filepath.Join(tmpDir, "auto.go")
	os.WriteFile(conflictedFile, []byte("<<<<<<< HEAD\nA\n=======\nB\n>>>>>>> feature\n"), 0644)

	// Mock Exec to return the detected file
	originalExecCommand := execCommand
	execCommand = func(name string, arg ...string) *exec.Cmd {
		if name == "git" && arg[0] == "diff" {
			// Return detected file
			return exec.Command("echo", "auto.go")
		}
		// Default fallback
		return exec.Command("true")
	}
	defer func() { execCommand = originalExecCommand }()

	// Mock Agent
	mockAgent := &ResolveSpyAgent{
		SendFunc: func(ctx context.Context, prompt string) (string, error) {
			return "RESOLVED", nil
		},
	}
	originalAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalAgentFactory }()

	// Execute without --file (triggers auto-detect)
	cmd := resolveCmd
	resolveFile = ""
	resolveAuto = true
	cmd.Flags().Parse([]string{"--auto"})

	err := cmd.RunE(cmd, []string{})
	assert.NoError(t, err)

	content, _ := os.ReadFile(conflictedFile)
	assert.Equal(t, "RESOLVED", string(content))
}
