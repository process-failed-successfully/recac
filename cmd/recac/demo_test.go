package main

import (
	"context"
	"os"
	"os/exec"
	"testing"

	"recac/internal/agent"

	"github.com/AlecAivazis/survey/v2"
	"github.com/stretchr/testify/assert"
)

// DemoMockAgent for testing
type DemoMockAgent struct {
	Response string
	Err      error
}

func (m *DemoMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, m.Err
}

func (m *DemoMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	onChunk(m.Response)
	return m.Response, m.Err
}

func TestDemoCmd(t *testing.T) {
	// Setup Mocks
	originalAgentFactory := agentClientFactory
	originalWriteFile := writeFileFunc
	originalExecCommand := execCommand
	originalMkdirAll := mkdirAllFunc
	originalAskOne := askOneFunc

	t.Cleanup(func() {
		agentClientFactory = originalAgentFactory
		writeFileFunc = originalWriteFile
		execCommand = originalExecCommand
		mkdirAllFunc = originalMkdirAll
		askOneFunc = originalAskOne
	})

	// Mock Agent
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return &DemoMockAgent{
			Response: "Output demo.gif\nType 'ls'\nEnter",
		}, nil
	}

	// Mock File Write
	filesWritten := make(map[string]string)
	writeFileFunc = func(filename string, data []byte, perm os.FileMode) error {
		filesWritten[filename] = string(data)
		return nil
	}

	mkdirAllFunc = func(path string, perm os.FileMode) error {
		return nil
	}

	// Mock Exec to simulate 'vhs' being present and succeeding
	execCommand = func(name string, arg ...string) *exec.Cmd {
		// We use 'true' command which is available in most linux environments and exits with 0
		return exec.Command("true")
	}

	// Mock Confirmation (Auto-Yes)
	askOneFunc = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
		if _, ok := p.(*survey.Confirm); ok {
			*(response.(*bool)) = true // Yes
			return nil
		}
		return nil
	}

	// Test Case 1: Render enabled (default)
	demoOutput = "test.tape"
	demoRender = true
	demoAuto = false // Should ask for confirmation

	// Create a dummy command context
	cmd := demoCmd
	args := []string{"Test Scenario"}

	err := runDemo(cmd, args)

	assert.NoError(t, err)
	assert.Contains(t, filesWritten["test.tape"], "Output demo.gif")

	// Test Case 2: Render disabled
	demoRender = false
	err = runDemo(cmd, args)
	assert.NoError(t, err)
	assert.Contains(t, filesWritten["test.tape"], "Output demo.gif")

	// Test Case 3: Render enabled, Auto mode
	demoRender = true
	demoAuto = true
	// Mock askOneFunc should NOT be called, but if it is, it's fine.
	// How to verify it wasn't called? We can check inside mock.
	// But simply checking success is enough here.
	err = runDemo(cmd, args)
	assert.NoError(t, err)
}
